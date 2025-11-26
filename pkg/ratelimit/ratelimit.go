package ratelimit

import (
	"fmt"

	sensor "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
)

const (
	secondsInMinute float64 = 60
	secondsInHour   float64 = 3600
)

type RateLimitCalculator struct {
	defaultRateLimit sensor.RateLimit
}

func NewRateLimitCalculatorOrDie(defaultUnit string, defaultLimitValue int32) *RateLimitCalculator {
	if !validRateLimitUnit(defaultUnit) {
		panic(fmt.Errorf("invalid defaultUnit: %s", defaultUnit))
	}

	return &RateLimitCalculator{
		defaultRateLimit: sensor.RateLimit{
			Unit:            sensor.RateLimiteUnit(defaultUnit),
			RequestsPerUnit: defaultLimitValue,
		},
	}
}

// Calculates the RateLimit based on min(sensorValue, maxRateLimit) where
// maxRateLimit is the namespaceValue if set, otherwise defaultRateLimit.
func (r *RateLimitCalculator) Calculate(namespaceValue, sensorValue *sensor.RateLimit) sensor.RateLimit {
	maxRateLimit := r.defaultRateLimit
	if namespaceValue != nil {
		// Namespace value overrides the default
		maxRateLimit = *namespaceValue
	}

	if sensorValue == nil {
		return maxRateLimit
	}

	return min(maxRateLimit, *sensorValue)
}

func validRateLimitUnit(unit string) bool {
	rateLimitUnit := sensor.RateLimiteUnit(unit)

	switch rateLimitUnit {
	case sensor.Second, sensor.Minute, sensor.Hour:
		return true
	default:
		return false
	}
}

func min(a, b sensor.RateLimit) sensor.RateLimit {
	aRequestsPerSecond := calculateRequestsPerSecond(a)
	bRequestsPerSecond := calculateRequestsPerSecond(b)

	if aRequestsPerSecond < bRequestsPerSecond {
		return a
	}
	return b
}

func calculateRequestsPerSecond(input sensor.RateLimit) float64 {
	switch input.Unit {
	case sensor.Second:
		return float64(input.RequestsPerUnit)
	case sensor.Minute:
		return float64(input.RequestsPerUnit) / secondsInMinute
	case sensor.Hour:
		return float64(input.RequestsPerUnit) / secondsInHour
	default:
		return float64(input.RequestsPerUnit) // defaults to Second if the input unit is invalid
	}
}
