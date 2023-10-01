package v1

const (
	ConditionUploaded = "Uploaded"
	ConditionBuilt    = "Built"
	ConditionComplete = "Complete"
	ConditionServing  = "Serving"
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
	ReasonJobFailed          = "JobFailed"
	ReasonDeploymentReady    = "DeploymentReady"
	ReasonDeploymentNotReady = "DeploymentNotReady"
	ReasonPodReady           = "PodReady"
	ReasonPodNotReady        = "PodNotReady"

	ReasonSuspended = "Suspended"

	ReasonAwaitingUpload = "AwaitingUpload"
	ReasonUploadFound    = "UploadFound"
)
