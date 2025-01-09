package entities

type ComponentName string

const (
	ComponentNameAdminAPI        = ComponentName("AdminAPI")
	ComponentNameLifeCycleWorker = ComponentName("ExpiryWorkflow")
	ComponentNamePurgeWorkflow   = ComponentName("PurgeWorkflow")
)
