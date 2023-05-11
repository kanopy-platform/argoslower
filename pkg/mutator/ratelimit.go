package mutator

import (
	"fmt"
	"math"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
)

const (
	secondsInHour = 3600
	minutesInHour = 60
)

type RateLimitCalculator struct {
	defaultRateLimit sensor.RateLimit
}

func NewRateLimitCalculator(defaultRequestsPerHour int32) *RateLimitCalculator {
	return &RateLimitCalculator{
		defaultRateLimit: sensor.RateLimit{
			Unit:            sensor.Hour,
			RequestsPerUnit: defaultRequestsPerHour,
		},
	}
}

// Calculates the RateLimit based on min(sensorValue, maxRateLimit) where
// maxRateLimit is the namespaceValue if set, otherwise defaultRateLimit.
// Returns a value in requests per hour.
func (r *RateLimitCalculator) Calculate(namespaceValue, sensorValue *sensor.RateLimit) (sensor.RateLimit, error) {
	maxRateLimit := r.defaultRateLimit
	if namespaceValue != nil {
		// Namespace value overrides the default
		maxRateLimit = *namespaceValue
	}

	normalizedMaxRateLimit, err := normalizeToHour(maxRateLimit)
	if err != nil {
		return sensor.RateLimit{}, err
	}

	if sensorValue == nil {
		return normalizedMaxRateLimit, nil
	}

	// Compare normalized values
	normalizedSensorValue, err := normalizeToHour(*sensorValue)
	if err != nil {
		return sensor.RateLimit{}, err
	}

	return min(normalizedMaxRateLimit, normalizedSensorValue)
}

func normalizeToHour(input sensor.RateLimit) (sensor.RateLimit, error) {
	switch input.Unit {
	case sensor.Second:
		if resultWillOverflowInt32(input.RequestsPerUnit, secondsInHour) {
			return sensor.RateLimit{}, fmt.Errorf("input RateLimit (unit: %s, requests: %d) will overflow", input.Unit, input.RequestsPerUnit)
		}

		return sensor.RateLimit{
			Unit:            sensor.Hour,
			RequestsPerUnit: input.RequestsPerUnit * secondsInHour,
		}, nil

	case sensor.Minute:
		if resultWillOverflowInt32(input.RequestsPerUnit, minutesInHour) {
			return sensor.RateLimit{}, fmt.Errorf("input RateLimit (unit: %s, requests: %d) will overflow", input.Unit, input.RequestsPerUnit)
		}

		return sensor.RateLimit{
			Unit:            sensor.Hour,
			RequestsPerUnit: input.RequestsPerUnit * minutesInHour,
		}, nil

	case sensor.Hour:
		return input, nil

	default:
		return sensor.RateLimit{}, fmt.Errorf("invalid RateLimit unit: %s", input.Unit)
	}
}

func resultWillOverflowInt32(value int32, multiplier int) bool {
	return float64(value) > (float64(math.MaxInt32) / float64(multiplier))
}

func min(a, b sensor.RateLimit) (sensor.RateLimit, error) {
	if a.Unit != b.Unit {
		return sensor.RateLimit{}, fmt.Errorf("mismatched units for RateLimit comparison: %s, %s", a.Unit, b.Unit)
	}

	if a.RequestsPerUnit < b.RequestsPerUnit {
		return a, nil
	}
	return b, nil
}
