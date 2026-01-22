package v1alpha1

// Action is the action that Workflow controller will take according to the event.
type Action string

const (
	// SyncWorkflowAction is the action to sync Workflow status.
	SyncWorkflowAction Action = "SyncWorkflow"
	// SyncWorkTemplateAction is the action to sync WorkTemplate status.
	SyncWorkTemplateAction Action = "SyncWorkTemplate"
)
