// Package runtime owns the Docker-facing worker provider contract.
package runtime

import (
	"context"
	"io"
	"time"
)

type InstanceSpec struct {
	InstanceID, ChallengeID, ImageRef, ImageDigest, DynamicFlag, PublicHost, BindHost, BaseDomain, Protocol string
	RegistryHost, RegistryUsername, RegistryToken                                                           string
	HostPort, InternalPort, CPUMilli, MemoryMB, PIDsLimit, DiskMB                                           int
	ReadOnlyRootFS                                                                                          bool
	Environment                                                                                             map[string]string
	Labels                                                                                                  map[string]string
}
type RuntimeInstance struct {
	ID, NetworkID, AccessURL string
	HostPort, InternalPort   int
	Status                   string
	StartedAt                time.Time
}
type RuntimeStatus struct {
	Status, Health, Error string
	Running               bool
	StartedAt, FinishedAt *time.Time
}
type LogOptions struct {
	Tail  int
	Since time.Time
}
type CleanupResult struct {
	Containers, Networks int
}
type Provider interface {
	Start(context.Context, InstanceSpec) (RuntimeInstance, error)
	Restart(context.Context, string) error
	Stop(context.Context, string) error
	Reset(context.Context, RuntimeInstance, InstanceSpec) (RuntimeInstance, error)
	Inspect(context.Context, string) (RuntimeStatus, error)
	Logs(context.Context, string, LogOptions) (io.ReadCloser, error)
	CleanupOrphans(context.Context, map[string]bool) (CleanupResult, error)
	CachedImages(context.Context) ([]string, error)
}

type Error struct {
	Code, Message string
	Cause         error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
