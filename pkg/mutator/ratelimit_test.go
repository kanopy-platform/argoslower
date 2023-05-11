package mutator

import (
	"math"
	"testing"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestMCalculate(t *testing.T) {
	t.Parallel()

	defaultRateLimit := sensor.RateLimit{
		Unit:            sensor.Hour,
		RequestsPerUnit: 100,
	}

	tests := []struct {
		testMsg        string
		namespaceValue *sensor.RateLimit
		sensorValue    *sensor.RateLimit
		wantResult     sensor.RateLimit
		wantError      bool
	}{
		{
			testMsg:        "no namespaceValue and no sensorValue, use normalized default",
			namespaceValue: nil,
			sensorValue:    nil,
			wantResult:     defaultRateLimit,
			wantError:      false,
		},
		{
			testMsg:        "namespaceValue set but no sensorValue, use normalized namespaceValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			sensorValue:    nil,
			wantResult:     sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 1 * secondsInHour},
			wantError:      false,
		},
		{
			testMsg:        "sensorValue < [default, namespaceValue], use normalized sensorValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			sensorValue:    &sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 1},
			wantResult:     sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 60},
			wantError:      false,
		},
		{
			testMsg:        "sensorValue > [default, namespaceValue], use normalized namespaceValue",
			namespaceValue: &sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 1000},
			sensorValue:    &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			wantResult:     sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 1000},
			wantError:      false,
		},
		{
			testMsg:        "sensorValue > default, namespaceValue unset, use normalized default",
			namespaceValue: nil,
			sensorValue:    &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 1},
			wantResult:     sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 100},
			wantError:      false,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)
		r := NewRateLimitCalculator(defaultRateLimit.RequestsPerUnit)

		result, err := r.Calculate(test.namespaceValue, test.sensorValue)
		assert.Equal(t, test.wantResult, result)
		assert.Equal(t, test.wantError, err != nil)
	}
}

func TestNormalizeToHour(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testMsg    string
		input      sensor.RateLimit
		wantResult sensor.RateLimit
		wantError  bool
	}{
		{
			testMsg:    "seconds",
			input:      sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
			wantResult: sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 10 * secondsInHour},
			wantError:  false,
		},
		{
			testMsg:    "minutes",
			input:      sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 10},
			wantResult: sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 10 * minutesInHour},
			wantError:  false,
		},
		{
			testMsg:    "hours",
			input:      sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 10},
			wantResult: sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 10},
			wantError:  false,
		},
		{
			testMsg:    "invalid unit",
			input:      sensor.RateLimit{Unit: "", RequestsPerUnit: 10},
			wantResult: sensor.RateLimit{},
			wantError:  true,
		},
		{
			testMsg:    "overflow",
			input:      sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: math.MaxInt32},
			wantResult: sensor.RateLimit{},
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result, err := normalizeToHour(test.input)
		assert.Equal(t, test.wantResult, result)
		assert.Equal(t, test.wantError, err != nil)
	}
}

func TestResultWillOverflowInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testMsg    string
		value      int32
		multiplier int
		want       bool
	}{
		{
			testMsg:    "no overflow",
			value:      int32(math.MaxInt32 / 60),
			multiplier: 60,
			want:       false,
		},
		{
			testMsg:    "overflow",
			value:      int32(math.MaxInt32/60) + 1,
			multiplier: 60,
			want:       true,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result := resultWillOverflowInt32(test.value, test.multiplier)
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
		wantError  bool
	}{
		{
			testMsg:    "a < b",
			a:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
			b:          sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 11},
			wantResult: sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 10},
			wantError:  false,
		},
		{
			testMsg:    "a > b",
			a:          sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 790},
			b:          sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 789},
			wantResult: sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 789},
			wantError:  false,
		},
		{
			testMsg:    "mismatched units",
			a:          sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 790},
			b:          sensor.RateLimit{Unit: sensor.Hour, RequestsPerUnit: 789},
			wantResult: sensor.RateLimit{},
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)

		result, err := min(test.a, test.b)
		assert.Equal(t, test.wantResult, result)
		assert.Equal(t, test.wantError, err != nil)
	}
}
