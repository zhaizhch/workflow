package v1alpha1

// Event represent the phase of Workflow
type Event string

const (
	// OutOfSyncEvent is triggered if Workflow is updated(add/update/delete)
	OutOfSyncEvent Event = "OutOfSync"
)
