package instance

import "testing"

func TestLifecycleOperationsAreDistinct(t *testing.T) {
	tests := []struct {
		operation string
		from, to  Status
	}{{"start", Stopped, Pending}, {"restart", Running, Restarting}, {"stop", Running, Stopping}, {"reset", Running, Resetting}}
	seen := map[Status]bool{}
	for _, test := range tests {
		got, err := Next(test.operation, test.from)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.to {
			t.Fatalf("%s: got %s want %s", test.operation, got, test.to)
		}
		if seen[got] {
			t.Fatalf("transition target reused for %s", test.operation)
		}
		seen[got] = true
	}
	if _, err := Next("reset", Stopped); err == nil {
		t.Fatal("reset from stopped must fail")
	}
	if _, err := Next("restart", Failed); err == nil {
		t.Fatal("restart from failed must fail")
	}
}

func TestDirectTransitionWhitelist(t *testing.T) {
	allowed := [][2]Status{{Pending, Pulling}, {Pulling, Creating}, {Creating, Starting}, {Starting, Running}, {Running, Resetting}, {Stopping, Stopped}}
	for _, pair := range allowed {
		if !CanTransition(pair[0], pair[1]) {
			t.Fatalf("expected %s -> %s to be allowed", pair[0], pair[1])
		}
	}
	if CanTransition(Stopped, Running) || CanTransition(Deleted, Running) {
		t.Fatal("terminal or stopped states must not jump directly to running")
	}
}
