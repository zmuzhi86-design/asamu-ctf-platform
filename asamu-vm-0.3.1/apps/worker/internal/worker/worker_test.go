package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/models"
	runtimeprovider "asamu.local/platform/api/worker/internal/runtime"
)

func TestPortProtocolSeparatesUDPFromTCPBackedProtocols(t *testing.T) {
	tests := map[string]string{"udp": "udp", "UDP": "udp", "tcp": "tcp", "http": "tcp", "https": "tcp", "": "tcp"}
	for input, expected := range tests {
		if actual := portProtocol(input); actual != expected {
			t.Fatalf("portProtocol(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestHeartbeatLoopStopsWhenWorkerContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Worker{}).runHeartbeat(ctx, time.Hour)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not stop after cancellation")
	}
}

func TestContainsStringUsesExactCachedImageReference(t *testing.T) {
	images := json.RawMessage(`["registry.example/lab@sha256:abc","postgres:16-alpine"]`)
	if !containsString(images, "postgres:16-alpine") {
		t.Fatal("cached image was not found")
	}
	if containsString(images, "postgres:16") || containsString(json.RawMessage(`invalid`), "postgres:16-alpine") {
		t.Fatal("cache matching must require a valid exact reference")
	}
}

func TestCapacityAllowsStartAndHonorsDrain(t *testing.T) {
	node := models.RuntimeWorkerNode{Enabled: true, Status: "online", CPUTotalMilli: 2000, MemoryTotalMB: 2048, MaxInstances: 3, LastHeartbeat: time.Now().UTC()}
	job := scheduledJob{Protocol: "tcp", CPUMilli: 500, MemoryMB: 256}
	allowed, _, err := capacityAllows(node, workerUsage{ActiveInstances: 1, CPUMilli: 1000, MemoryMB: 1024}, job, []string{"http", "tcp"})
	if err != nil || !allowed {
		t.Fatalf("valid capacity rejected: allowed=%v err=%v", allowed, err)
	}
	node.Draining = true
	if allowed, reason, _ := capacityAllows(node, workerUsage{}, job, []string{"tcp"}); allowed || reason != "worker_not_accepting_starts" {
		t.Fatalf("draining worker accepted start: allowed=%v reason=%s", allowed, reason)
	}
	node.Draining = false
	if allowed, reason, _ := capacityAllows(node, workerUsage{ActiveInstances: 3}, job, []string{"tcp"}); allowed || reason != "instance_capacity_exhausted" {
		t.Fatalf("full worker accepted start: allowed=%v reason=%s", allowed, reason)
	}
}

func TestPortAllocationFailureDoesNotMaskDatabaseErrorsAsExhaustion(t *testing.T) {
	w := &Worker{cfg: config.Runtime{PortMin: 20000, PortMax: 20049}}
	failure := w.failPortAllocation(models.InstanceRuntimeJob{}, models.ChallengeInstance{}, errors.New("database unavailable")).(*operationFailure)
	if failure.code != "PORT_ALLOCATION_FAILED" || !failure.retryable {
		t.Fatalf("database error classified as code=%q retryable=%v", failure.code, failure.retryable)
	}

	failure = w.failPortAllocation(models.InstanceRuntimeJob{}, models.ChallengeInstance{}, fmt.Errorf("retry: %w", errRuntimePortPoolExhausted)).(*operationFailure)
	if failure.code != "PORT_EXHAUSTED" || !strings.Contains(failure.message, "20000-20049") {
		t.Fatalf("pool exhaustion classified as code=%q message=%q", failure.code, failure.message)
	}
}

func TestRuntimeMissingDistinguishesNotFoundFromTransientInspectFailure(t *testing.T) {
	missing, err := runtimeMissing(runtimeprovider.RuntimeStatus{}, &runtimeprovider.Error{Code: "CONTAINER_NOT_FOUND", Message: "gone"})
	if err != nil || !missing {
		t.Fatalf("not-found runtime was not treated as missing: missing=%v err=%v", missing, err)
	}
	transient := &runtimeprovider.Error{Code: "CONTAINER_INSPECT_FAILED", Message: "docker unavailable"}
	missing, err = runtimeMissing(runtimeprovider.RuntimeStatus{}, transient)
	if missing || !errors.Is(err, transient) {
		t.Fatalf("transient inspect error was treated as drift: missing=%v err=%v", missing, err)
	}
	missing, err = runtimeMissing(runtimeprovider.RuntimeStatus{Running: true}, nil)
	if err != nil || missing {
		t.Fatalf("running runtime was treated as missing: missing=%v err=%v", missing, err)
	}
}

func TestHostPortConflictIsRetryable(t *testing.T) {
	if !runtimeErrorRetryable("HOST_PORT_CONFLICT") {
		t.Fatal("host port conflicts must retry with a different allocated port")
	}
}

func TestRuntimePortPoolSeriesUsesExplicitIntegerTypes(t *testing.T) {
	if strings.Count(syncRuntimePortPoolSQL, "?::integer") != 2 {
		t.Fatal("generate_series parameters must be explicitly typed for PostgreSQL")
	}
}
