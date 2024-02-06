package testing

import (
	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
)

type FakeRateLimitGetter struct {
	Rates map[string]*sensor.RateLimit
	Err   error
}

func (frlg *FakeRateLimitGetter) RateLimit(namespace string) (*sensor.RateLimit, error) {
	if r, ok := frlg.Rates[namespace]; ok {
		return r, nil
	} else {
		return nil, frlg.Err
	}
}

func NewFakeRate() FakeRateLimitGetter {
	return FakeRateLimitGetter{
		Rates: map[string]*sensor.RateLimit{},
	}
}
