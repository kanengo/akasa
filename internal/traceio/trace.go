package traceio

import "go.opentelemetry.io/otel/attribute"

const (
	AppTraceKey          = attribute.Key("serviceakasar.app")
	DeploymentIdTraceKey = attribute.Key("serviceakasar.deployment_id")
	AkasaletIdTraceKey   = attribute.Key("serviceakasar.akasalet_id")
)

func App(app string) attribute.KeyValue {
	return AppTraceKey.String(app)
}

func DeploymentId(deploymentId string) attribute.KeyValue {
	return DeploymentIdTraceKey.String(deploymentId)
}

func AkasaletId(akasaletId string) attribute.KeyValue {
	return AkasaletIdTraceKey.String(akasaletId)
}
