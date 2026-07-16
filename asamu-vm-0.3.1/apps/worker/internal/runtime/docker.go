// Package runtime contains Docker SDK integration and is built only into the worker.
package runtime

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type DockerProvider struct {
	client            *client.Client
	allowed           map[string]bool
	pullMissingImages bool
}

func (d *DockerProvider) CachedImages(ctx context.Context) ([]string, error) {
	images, err := d.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, &Error{Code: "IMAGE_LIST_FAILED", Message: "unable to list images on the Docker host", Cause: err}
	}
	return cachedImageRefs(images), nil
}

func cachedImageRefs(images []image.Summary) []string {
	seen := map[string]bool{}
	for _, item := range images {
		refs := append(append([]string{}, item.RepoTags...), item.RepoDigests...)
		for _, ref := range refs {
			if ref != "" && ref != "<none>:<none>" && ref != "<none>@<none>" {
				seen[ref] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for ref := range seen {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result
}

func NewDockerProvider(host string, allowedImages []string, pullMissingImages bool) (*DockerProvider, error) {
	options := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if host != "" {
		options = append(options, client.WithHost(host))
	}
	cli, err := client.NewClientWithOpts(options...)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, value := range allowedImages {
		if value = strings.TrimSpace(value); value != "" {
			allowed[value] = true
		}
	}
	return &DockerProvider{client: cli, allowed: allowed, pullMissingImages: pullMissingImages}, nil
}
func (d *DockerProvider) validateImage(spec InstanceSpec) error {
	if strings.TrimSpace(spec.ImageRef) == "" {
		return &Error{Code: "INVALID_IMAGE", Message: "challenge image is empty"}
	}
	if spec.ImageDigest != "" && !strings.HasSuffix(spec.ImageRef, "@"+spec.ImageDigest) {
		return &Error{Code: "IMAGE_DIGEST_REQUIRED", Message: "runtime image must use a pinned digest when a digest is configured"}
	}
	return nil
}
func (d *DockerProvider) ensureImage(ctx context.Context, spec InstanceSpec) error {
	if _, _, err := d.client.ImageInspectWithRaw(ctx, spec.ImageRef); err == nil {
		return nil
	} else if !client.IsErrNotFound(err) {
		return &Error{Code: "IMAGE_INSPECT_FAILED", Message: "unable to inspect images on the Docker host", Cause: err}
	}
	if !d.pullMissingImages {
		return &Error{Code: "IMAGE_NOT_PRESENT", Message: "challenge image is not present on the Docker host and automatic pulling is disabled"}
	}
	if !d.allowed[spec.ImageRef] {
		return &Error{Code: "IMAGE_PULL_NOT_ALLOWED", Message: "automatic pulling is allowed only for an exact image reference in RUNTIME_ALLOWED_IMAGES"}
	}
	pullOptions := image.PullOptions{}
	if spec.RegistryToken != "" {
		auth, err := registry.EncodeAuthConfig(registry.AuthConfig{Username: spec.RegistryUsername, Password: spec.RegistryToken, ServerAddress: spec.RegistryHost})
		if err != nil {
			return &Error{Code: "REGISTRY_AUTH_ENCODE_FAILED", Message: "unable to prepare private registry authentication", Cause: err}
		}
		pullOptions.RegistryAuth = auth
	}
	stream, err := d.client.ImagePull(ctx, spec.ImageRef, pullOptions)
	if err != nil {
		return &Error{Code: "IMAGE_PULL_FAILED", Message: "unable to pull challenge image", Cause: err}
	}
	defer stream.Close()
	if _, err = io.Copy(io.Discard, stream); err != nil {
		return &Error{Code: "IMAGE_PULL_FAILED", Message: "unable to read the Docker image pull response", Cause: err}
	}
	if _, _, err = d.client.ImageInspectWithRaw(ctx, spec.ImageRef); err != nil {
		return &Error{Code: "IMAGE_PULL_FAILED", Message: "Docker did not make the pulled image available", Cause: err}
	}
	return nil
}
func (d *DockerProvider) Start(ctx context.Context, spec InstanceSpec) (RuntimeInstance, error) {
	if err := d.validateImage(spec); err != nil {
		return RuntimeInstance{}, err
	}
	if err := d.ensureImage(ctx, spec); err != nil {
		return RuntimeInstance{}, err
	}
	protocol := strings.ToLower(spec.Protocol)
	if protocol != "udp" {
		protocol = "tcp"
	}
	port := nat.Port(strconv.Itoa(spec.InternalPort) + "/" + protocol)
	bindHost := spec.BindHost
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}
	hostPort := strconv.Itoa(spec.HostPort)
	name := "asamu-" + strings.ReplaceAll(spec.InstanceID, "-", "")
	if len(name) > 48 {
		name = name[:48]
	}
	if existing, err := d.client.ContainerInspect(ctx, name); err == nil {
		if existing.Config == nil || existing.Config.Labels["asamu.managed"] != "true" || existing.Config.Labels["asamu.instance"] != spec.InstanceID {
			return RuntimeInstance{}, &Error{Code: "CONTAINER_NAME_CONFLICT", Message: "existing container is not owned by this challenge instance"}
		}
		configuredBindings := nat.PortMap(nil)
		if existing.HostConfig != nil {
			configuredBindings = existing.HostConfig.PortBindings
		}
		activeBindings := nat.PortMap(nil)
		if existing.NetworkSettings != nil {
			activeBindings = existing.NetworkSettings.Ports
		}
		if !portBindingMatches(configuredBindings, port, bindHost, hostPort) || !portBindingMatches(activeBindings, port, bindHost, hostPort) {
			if err := d.Stop(ctx, existing.ID); err != nil {
				return RuntimeInstance{}, &Error{Code: "CONTAINER_RECOVERY_FAILED", Message: "unable to replace a challenge container with an inactive port mapping", Cause: err}
			}
		} else {
			if existing.State != nil && !existing.State.Running {
				if err := d.client.ContainerStart(ctx, existing.ID, container.StartOptions{}); err != nil {
					return RuntimeInstance{}, &Error{Code: "CONTAINER_RECOVERY_FAILED", Message: "unable to recover existing challenge container", Cause: err}
				}
			}
			networkID := ""
			if existing.NetworkSettings != nil {
				for _, settings := range existing.NetworkSettings.Networks {
					if settings.NetworkID != "" {
						networkID = settings.NetworkID
						break
					}
				}
			}
			startedAt := time.Now().UTC()
			if existing.State != nil && existing.State.StartedAt != "" {
				if parsed, err := time.Parse(time.RFC3339Nano, existing.State.StartedAt); err == nil {
					startedAt = parsed
				}
			}
			return RuntimeInstance{ID: existing.ID, NetworkID: networkID, AccessURL: accessURL(spec), HostPort: spec.HostPort, InternalPort: spec.InternalPort, Status: "running", StartedAt: startedAt}, nil
		}
	} else if !client.IsErrNotFound(err) {
		return RuntimeInstance{}, &Error{Code: "CONTAINER_INSPECT_FAILED", Message: "unable to inspect existing challenge container", Cause: err}
	}
	networkName := name + "-net"
	networkID := ""
	if existing, err := d.client.NetworkInspect(ctx, networkName, network.InspectOptions{}); err == nil {
		if existing.Labels["asamu.kind"] != "range-network" || existing.Labels["asamu.instance"] != spec.InstanceID {
			return RuntimeInstance{}, &Error{Code: "NETWORK_NAME_CONFLICT", Message: "existing network is not owned by this challenge instance"}
		}
		if existing.Internal {
			if err := d.client.NetworkRemove(ctx, existing.ID); err != nil && !client.IsErrNotFound(err) {
				return RuntimeInstance{}, &Error{Code: "NETWORK_REMOVE_FAILED", Message: "unable to replace an internal challenge network", Cause: err}
			}
		} else {
			networkID = existing.ID
		}
	} else if client.IsErrNotFound(err) {
	} else {
		return RuntimeInstance{}, &Error{Code: "NETWORK_INSPECT_FAILED", Message: "unable to inspect isolated challenge network", Cause: err}
	}
	if networkID == "" {
		created, createErr := d.client.NetworkCreate(ctx, networkName, rangeNetworkCreateOptions(spec))
		if createErr != nil {
			return RuntimeInstance{}, &Error{Code: "NETWORK_CREATE_FAILED", Message: "unable to create isolated challenge network", Cause: createErr}
		}
		networkID = created.ID
	}
	cleanupNetwork := true
	defer func() {
		if cleanupNetwork {
			_ = d.client.NetworkRemove(context.Background(), networkID)
		}
	}()
	pids := int64(spec.PIDsLimit)
	env := []string{
		"ASAMU_FLAG=" + spec.DynamicFlag,
		"ASAMU_INSTANCE_ID=" + spec.InstanceID,
		// Keep the legacy names for challenge images built before the rename.
		"CM_FLAG=" + spec.DynamicFlag,
		"CM_INSTANCE_ID=" + spec.InstanceID,
	}
	for key, value := range spec.Environment {
		if key != "ASAMU_FLAG" && key != "ASAMU_INSTANCE_ID" && key != "CM_FLAG" && key != "CM_INSTANCE_ID" {
			env = append(env, key+"="+value)
		}
	}
	labels := merge(spec.Labels, map[string]string{"asamu.managed": "true", "asamu.instance": spec.InstanceID, "asamu.challenge": spec.ChallengeID})
	config := &container.Config{Image: spec.ImageRef, Env: env, Labels: labels, ExposedPorts: nat.PortSet{port: struct{}{}}}
	hostConfig := &container.HostConfig{AutoRemove: false, ReadonlyRootfs: spec.ReadOnlyRootFS, SecurityOpt: []string{"no-new-privileges:true"}, CapDrop: []string{"ALL"}, NetworkMode: container.NetworkMode(networkName), PortBindings: nat.PortMap{port: []nat.PortBinding{{HostIP: bindHost, HostPort: hostPort}}}, Resources: container.Resources{Memory: int64(spec.MemoryMB) * 1024 * 1024, NanoCPUs: int64(spec.CPUMilli) * 1_000_000, PidsLimit: &pids}, Tmpfs: map[string]string{"/tmp": "rw,noexec,nosuid,size=64m"}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "10m", "max-file": "2"}}}
	created, err := d.client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		if isHostPortConflict(err) {
			return RuntimeInstance{}, &Error{Code: "HOST_PORT_CONFLICT", Message: "the allocated host port is already in use", Cause: err}
		}
		return RuntimeInstance{}, &Error{Code: "CONTAINER_CREATE_FAILED", Message: "unable to create challenge container", Cause: err}
	}
	cleanupContainer := true
	defer func() {
		if cleanupContainer {
			_ = d.client.ContainerRemove(context.Background(), created.ID, container.RemoveOptions{Force: true})
		}
	}()
	if err := d.client.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return RuntimeInstance{}, &Error{Code: "CONTAINER_START_FAILED", Message: "unable to start challenge container", Cause: err}
	}
	cleanupContainer = false
	cleanupNetwork = false
	return RuntimeInstance{ID: created.ID, NetworkID: networkID, AccessURL: accessURL(spec), HostPort: spec.HostPort, InternalPort: spec.InternalPort, Status: "running", StartedAt: time.Now().UTC()}, nil
}

func rangeNetworkCreateOptions(spec InstanceSpec) network.CreateOptions {
	return network.CreateOptions{
		Driver:     "bridge",
		Internal:   false,
		Attachable: false,
		Labels:     merge(spec.Labels, map[string]string{"asamu.kind": "range-network", "asamu.instance": spec.InstanceID}),
	}
}

func portBindingMatches(bindings nat.PortMap, port nat.Port, hostIP, hostPort string) bool {
	for _, binding := range bindings[port] {
		if binding.HostPort == hostPort && (binding.HostIP == hostIP || (hostIP == "0.0.0.0" && binding.HostIP == "")) {
			return true
		}
	}
	return false
}

func isHostPortConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "port is already allocated") ||
		strings.Contains(message, "address already in use") ||
		(strings.Contains(message, "bind for") && strings.Contains(message, "port"))
}

func accessURL(spec InstanceSpec) string {
	scheme := strings.ToLower(spec.Protocol)
	if scheme == "http" || scheme == "https" {
		return fmt.Sprintf("%s://%s:%d", scheme, spec.PublicHost, spec.HostPort)
	}
	return fmt.Sprintf("%s:%d", spec.PublicHost, spec.HostPort)
}
func (d *DockerProvider) Restart(ctx context.Context, runtimeID string) error {
	timeout := 10
	if err := d.client.ContainerRestart(ctx, runtimeID, container.StopOptions{Timeout: &timeout}); err != nil {
		return &Error{Code: "CONTAINER_RESTART_FAILED", Message: "unable to restart challenge container", Cause: err}
	}
	return nil
}
func (d *DockerProvider) Stop(ctx context.Context, runtimeID string) error {
	if runtimeID == "" {
		return nil
	}
	inspect, err := d.client.ContainerInspect(ctx, runtimeID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return err
	}
	timeout := 10
	_ = d.client.ContainerStop(ctx, runtimeID, container.StopOptions{Timeout: &timeout})
	if err := d.client.ContainerRemove(ctx, runtimeID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !client.IsErrNotFound(err) {
		return &Error{Code: "CONTAINER_REMOVE_FAILED", Message: "unable to remove challenge container", Cause: err}
	}
	for _, settings := range inspect.NetworkSettings.Networks {
		if settings.NetworkID != "" {
			_ = d.client.NetworkRemove(ctx, settings.NetworkID)
		}
	}
	return nil
}
func (d *DockerProvider) Reset(ctx context.Context, old RuntimeInstance, spec InstanceSpec) (RuntimeInstance, error) {
	if err := d.Stop(ctx, old.ID); err != nil {
		return RuntimeInstance{}, err
	}
	return d.Start(ctx, spec)
}
func (d *DockerProvider) Inspect(ctx context.Context, runtimeID string) (RuntimeStatus, error) {
	value, err := d.client.ContainerInspect(ctx, runtimeID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return RuntimeStatus{}, &Error{Code: "CONTAINER_NOT_FOUND", Message: "challenge container no longer exists", Cause: err}
		}
		return RuntimeStatus{}, &Error{Code: "CONTAINER_INSPECT_FAILED", Message: "unable to inspect challenge container", Cause: err}
	}
	status := RuntimeStatus{Status: value.State.Status, Health: "none", Running: value.State.Running, Error: value.State.Error}
	if value.State.Health != nil {
		status.Health = value.State.Health.Status
	}
	if value.State.StartedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value.State.StartedAt); err == nil {
			status.StartedAt = &parsed
		}
	}
	if value.State.FinishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value.State.FinishedAt); err == nil {
			status.FinishedAt = &parsed
		}
	}
	return status, nil
}
func (d *DockerProvider) Logs(ctx context.Context, runtimeID string, opts LogOptions) (io.ReadCloser, error) {
	tail := "200"
	if opts.Tail > 0 {
		tail = strconv.Itoa(opts.Tail)
	}
	since := ""
	if !opts.Since.IsZero() {
		since = strconv.FormatInt(opts.Since.Unix(), 10)
	}
	return d.client.ContainerLogs(ctx, runtimeID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: tail, Since: since, Timestamps: true})
}

func (d *DockerProvider) CleanupOrphans(ctx context.Context, activeInstances map[string]bool) (CleanupResult, error) {
	result := CleanupResult{}
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return result, &Error{Code: "CONTAINER_LIST_FAILED", Message: "unable to list managed challenge containers", Cause: err}
	}
	for _, item := range containers {
		instanceID, managed := managedInstanceID(item.Labels)
		if !managed {
			continue
		}
		if _, err := uuid.Parse(instanceID); err != nil || activeInstances[instanceID] {
			continue
		}
		if err := d.Stop(ctx, item.ID); err != nil {
			return result, err
		}
		result.Containers++
	}
	networks, err := d.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return result, &Error{Code: "NETWORK_LIST_FAILED", Message: "unable to list managed challenge networks", Cause: err}
	}
	for _, item := range networks {
		instanceID, managed := managedNetworkInstanceID(item.Labels)
		if !managed {
			continue
		}
		if _, err := uuid.Parse(instanceID); err != nil || activeInstances[instanceID] {
			continue
		}
		if err := d.client.NetworkRemove(ctx, item.ID); err != nil && !client.IsErrNotFound(err) {
			return result, &Error{Code: "NETWORK_REMOVE_FAILED", Message: "unable to remove orphan challenge network", Cause: err}
		}
		result.Networks++
	}
	return result, nil
}

func managedInstanceID(labels map[string]string) (string, bool) {
	if labels["asamu.managed"] == "true" {
		return labels["asamu.instance"], true
	}
	if labels["chainmirror.managed"] == "true" {
		return labels["chainmirror.instance"], true
	}
	return "", false
}

func managedNetworkInstanceID(labels map[string]string) (string, bool) {
	if labels["asamu.kind"] == "range-network" {
		return labels["asamu.instance"], true
	}
	if labels["chainmirror.kind"] == "range-network" {
		return labels["chainmirror.instance"], true
	}
	return "", false
}

func merge(left, right map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range left {
		result[key] = value
	}
	for key, value := range right {
		result[key] = value
	}
	return result
}
