package admission

import sensor "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"

type RateLimitGetter interface {
	RateLimit(namespace string) (*sensor.RateLimit, error)
}
