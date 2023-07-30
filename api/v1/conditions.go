package v1

// Substatus follows a convention of mapping 1:1 controller to condition type.

const (
	ConditionBuilt    = "Built"
	ConditionLoaded   = "Loaded"
	ConditionModelled = "Modelled"
	ConditionDeployed = "Deployed"
)

const (
	ReasonModelNotFound = "ModelNotFound"
	ReasonModelNotReady = "ModelNotReady"

	ReasonBaseModelNotFound = "BaseModelNotFound"
	ReasonBaseModelNotReady = "BaseModelNotReady"

	ReasonDatasetNotFound = "DatasetNotFound"
	ReasonDatasetNotReady = "ReasonDatasetNotReady"

	ReasonJobNotComplete     = "JobNotComplete"
	ReasonJobComplete        = "JobComplete"
	ReasonDeploymentReady    = "DeploymentReady"
	ReasonDeploymentNotReady = "DeploymentNotReady"
	ReasonPodReady           = "PodReady"
	ReasonPodNotReady        = "PodNotReady"

	ReasonSuspended = "Suspended"

	ReasonAwaitingUpload = "AwaitingUpload"
)
