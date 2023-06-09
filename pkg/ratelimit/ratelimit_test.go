package ratelimit

import (
	"testing"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestCalculate(t *testing.T) {
	t.Parallel()

	defaultRateLimit := sensor.RateLimit{
		Unit:            sensor.Second,
		RequestsPerUnit: 1,
	}

	tests := []struct {
		testMsg        string
		namespaceValue *sensor.RateLimit
		sensorValue    *sensor.RateLimit
		wantResult     sensor.RateLimit
	}{
		{
			testMsg:        "no namespaceValue and no sensorValue, use default",
			namespaceValue: nil,
			sensorValue:    nil,
			wantResult:     defaultRateLimit,
		},
		{
			testMsg:        "namespaceValue set but no sensorValue, use namespaceValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 5},
			sensorValue:    nil,
			wantResult:     sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 5},
		},
		{
			testMsg:        "sensorValue < default < namespaceValue, use sensorValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 5},
			sensorValue:    &sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 1},
			wantResult:     sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 1},
		},
		{
			testMsg:        "sensorValue > namespaceValue > default, use namespaceValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 5},
			sensorValue:    &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			wantResult:     sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 5},
		},
		{
			testMsg:        "sensorValue > default, namespaceValue unset, use default",
			namespaceValue: nil,
			sensorValue:    &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 100},
			wantResult:     sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)
		r := NewRateLimitCalculatorOrDie(string(defaultRateLimit.Unit), defaultRateLimit.RequestsPerUnit)

		result := r.Calculate(test.namespaceValue, test.sensorValue)
		assert.Equal(t, test.wantResult, result)
	}
}

func TestValidRateLimitUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testMsg string
		input   string
		want    bool
	}{
		{
			testMsg: "valid unit",
			input:   "Hour",
			want:    true,
		},
		{
			testMsg: "invalid unit",
			input:   "Month",
			want:    false,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result := validRateLimitUnit(test.input)
		assert.Equal(t, test.want, result)
	}
}

func TestMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testMsg    string
		a          sensor.RateLimit
		b          sensor.RateLimit
		wantResult sensor.RateLimit
	}{
		{
			testMsg:    "a < b, same units",
			a:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
			b:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 11},
			wantResult: sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
		},
		{
			testMsg:    "a < b, mismatched units",
			a:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			b:          sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 3601},
			wantResult: sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
		},
		{
			testMsg:    "a > b, mismatched units",
			a:          sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 10},
			b:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			wantResult: sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 10},
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result := min(test.a, test.b)
		assert.Equal(t, test.wantResult, result)
	}
}

func TestCalculateRequestsPerSecond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testMsg    string
		input      sensor.RateLimit
		wantResult float64
	}{
		{
			testMsg:    "requests per second",
			input:      sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
			wantResult: 10,
		},
		{
			testMsg:    "requests per minute",
			input:      sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 10},
			wantResult: float64(10) / secondsInMinute,
		},
		{
			testMsg:    "requests per hour",
			input:      sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 10},
			wantResult: float64(10) / secondsInHour,
		},
		{
			testMsg:    "invalid unit, assumes unit=Second",
			input:      sensor.RateLimit{Unit: "", RequestsPerUnit: 10},
			wantResult: 10,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result := calculateRequestsPerSecond(test.input)
		assert.Equal(t, test.wantResult, result)
	}
}
