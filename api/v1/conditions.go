package v1

const (
	ConditionContainerReady    = "ContainerReady"
	ConditionDataReady         = "DataReady"
	ConditionModelReady        = "ModelReady"
	ConditionDependenciesReady = "DependenciesReady"
	ConditionChildrenReady     = "ChildrenReady"
)

const (
	ReasonJobNotComplete = "JobNotComplete"
	ReasonJobComplete    = "JobComplete"

	ReasonModelNotFound = "ModelNotFound"
	ReasonModelNotReady = "ModelNotReady"

	ReasonBaseModelNotFound = "BaseModelNotFound"
	ReasonBaseModelNotReady = "BaseModelNotReady"

	ReasonDatasetNotFound = "DatasetNotFound"
	ReasonDatasetNotReady = "ReasonDatasetNotReady"

	ReasonDeploymentNotReady = "DeploymentNotReady"
	ReasonPodNotReady        = "PodNotReady"

	ReasonSuspended = "Suspended"
)
