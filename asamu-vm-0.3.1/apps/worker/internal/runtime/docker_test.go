package runtime

import (
	"errors"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
)

func TestAccessURLPreservesSupportedProtocol(t *testing.T) {
	tests := map[string]string{
		"http":  "http://range.example:20000",
		"https": "https://range.example:20000",
		"tcp":   "range.example:20000",
		"udp":   "range.example:20000",
		"":      "range.example:20000",
	}
	for protocol, expected := range tests {
		actual := accessURL(InstanceSpec{Protocol: protocol, PublicHost: "range.example", HostPort: 20000})
		if actual != expected {
			t.Fatalf("accessURL(%q)=%q, want %q", protocol, actual, expected)
		}
	}
}

func TestHostPortConflictDetection(t *testing.T) {
	for _, message := range []string{
		"Bind for 127.0.0.1:20000 failed: port is already allocated",
		"listen tcp 127.0.0.1:20000: bind: address already in use",
	} {
		if !isHostPortConflict(errors.New(message)) {
			t.Fatalf("host port conflict was not detected: %s", message)
		}
	}
	if isHostPortConflict(errors.New("container create request timed out")) {
		t.Fatal("unrelated Docker error was classified as a host port conflict")
	}
}

func TestRangeNetworkAllowsPublishedHostPorts(t *testing.T) {
	options := rangeNetworkCreateOptions(InstanceSpec{InstanceID: "instance-1"})
	if options.Internal {
		t.Fatal("an internal Docker network prevents the configured host port from becoming active")
	}
	if options.Driver != "bridge" || options.Labels["asamu.instance"] != "instance-1" {
		t.Fatalf("unexpected range network options: %#v", options)
	}
}

func TestPortBindingMatchesConfiguredAndActiveMappings(t *testing.T) {
	port := nat.Port("9999/tcp")
	bindings := nat.PortMap{port: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "20000"}}}
	if !portBindingMatches(bindings, port, "0.0.0.0", "20000") {
		t.Fatal("valid published port mapping was not detected")
	}
	if portBindingMatches(bindings, port, "0.0.0.0", "20001") {
		t.Fatal("wrong host port was accepted")
	}
	if portBindingMatches(nat.PortMap{port: nil}, port, "0.0.0.0", "20000") {
		t.Fatal("an inactive runtime port mapping was accepted")
	}
}

func TestValidateImageAllowsLocalImagesWhenAllowlistIsEmpty(t *testing.T) {
	provider := &DockerProvider{allowed: map[string]bool{}}
	if err := provider.validateImage(InstanceSpec{ImageRef: "ctf-upload:latest"}); err != nil {
		t.Fatalf("local image should be accepted without an allowlist: %v", err)
	}
}

func TestValidateImageAllowsExistingLocalReferenceOutsidePullAllowlist(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ref := "registry.example/ctf/web@" + digest
	provider := &DockerProvider{allowed: map[string]bool{ref: true}}
	if err := provider.validateImage(InstanceSpec{ImageRef: ref, ImageDigest: digest}); err != nil {
		t.Fatalf("configured pinned image should be accepted: %v", err)
	}
	if err := provider.validateImage(InstanceSpec{ImageRef: "ctf-upload:latest"}); err != nil {
		t.Fatalf("a local image must not be rejected by the remote-pull allowlist: %v", err)
	}
	if err := provider.validateImage(InstanceSpec{ImageRef: ref, ImageDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}); err == nil {
		t.Fatal("mismatched digest should be rejected")
	}
}

func TestCachedImageRefsReturnsEveryUsableLocalTagAndDigest(t *testing.T) {
	got := cachedImageRefs([]image.Summary{
		{RepoTags: []string{"jx2025pwn:latest", "<none>:<none>"}, RepoDigests: []string{"jx2025pwn@sha256:abc"}},
		{RepoTags: []string{"asamu/api:0.1.0", "jx2025pwn:latest"}},
	})
	want := []string{"asamu/api:0.1.0", "jx2025pwn:latest", "jx2025pwn@sha256:abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cachedImageRefs()=%#v, want %#v", got, want)
	}
}

func TestManagedInstanceIDSupportsCurrentAndLegacyLabels(t *testing.T) {
	tests := []struct {
		labels map[string]string
		want   string
		ok     bool
	}{
		{labels: map[string]string{"asamu.managed": "true", "asamu.instance": "current"}, want: "current", ok: true},
		{labels: map[string]string{"chainmirror.managed": "true", "chainmirror.instance": "legacy"}, want: "legacy", ok: true},
		{labels: map[string]string{"unrelated": "true"}},
	}
	for _, test := range tests {
		got, ok := managedInstanceID(test.labels)
		if got != test.want || ok != test.ok {
			t.Fatalf("managedInstanceID(%v)=(%q,%v), want (%q,%v)", test.labels, got, ok, test.want, test.ok)
		}
	}
}
