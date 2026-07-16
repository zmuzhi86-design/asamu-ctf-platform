package instance

import "fmt"

type Status string

const (
	Pending     Status = "pending"
	Pulling     Status = "pulling"
	Creating    Status = "creating"
	Starting    Status = "starting"
	Running     Status = "running"
	Restarting  Status = "restarting"
	Resetting   Status = "resetting"
	Stopping    Status = "stopping"
	Stopped     Status = "stopped"
	Failed      Status = "failed"
	Expired     Status = "expired"
	Interrupted Status = "interrupted"
	Deleted     Status = "deleted"
)

var transitions = map[string]map[Status]Status{
	"start":   {Stopped: Pending, Failed: Pending, Expired: Pending, Interrupted: Pending},
	"restart": {Running: Restarting},
	"stop":    {Pending: Stopping, Pulling: Stopping, Creating: Stopping, Starting: Stopping, Running: Stopping, Restarting: Stopping, Resetting: Stopping},
	"reset":   {Running: Resetting},
	"extend":  {Running: Running},
	"expire":  {Running: Stopping},
}

func Next(operation string, current Status) (Status, error) {
	allowed, ok := transitions[operation]
	if !ok {
		return "", fmt.Errorf("unknown operation %s", operation)
	}
	next, ok := allowed[current]
	if !ok {
		return "", fmt.Errorf("operation %s is not allowed from %s", operation, current)
	}
	return next, nil
}
func Transitional(status Status) bool {
	return status == Pending || status == Pulling || status == Creating || status == Starting || status == Restarting || status == Resetting || status == Stopping
}

var directTransitions = map[Status]map[Status]bool{
	Pending:     {Pulling: true, Creating: true, Starting: true, Stopping: true, Failed: true, Interrupted: true},
	Pulling:     {Creating: true, Stopping: true, Failed: true, Interrupted: true},
	Creating:    {Starting: true, Stopping: true, Failed: true, Interrupted: true},
	Starting:    {Running: true, Stopping: true, Failed: true, Interrupted: true},
	Running:     {Restarting: true, Resetting: true, Stopping: true, Expired: true, Failed: true, Interrupted: true},
	Restarting:  {Running: true, Stopping: true, Failed: true, Interrupted: true},
	Resetting:   {Running: true, Stopping: true, Failed: true, Interrupted: true},
	Stopping:    {Stopped: true, Expired: true, Failed: true, Interrupted: true},
	Stopped:     {Pending: true, Deleted: true},
	Expired:     {Pending: true, Deleted: true},
	Failed:      {Pending: true, Stopping: true, Deleted: true},
	Interrupted: {Pending: true, Stopping: true, Deleted: true},
}

func CanTransition(from, to Status) bool { return directTransitions[from][to] }
