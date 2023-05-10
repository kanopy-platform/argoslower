package namespace

import sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"

type RateLimitGetter interface {
	RateLimit(namespace string) (*sensor.RateLimit, error)
}
