package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	"github.com/kanopy-platform/argoslower/pkg/ratelimit"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type fakeRateLimitGetter struct {
	Rates map[string]*sensor.RateLimit
	Err   error
}

func (frlg *fakeRateLimitGetter) RateLimit(namespace string) (*sensor.RateLimit, error) {
	if r, ok := frlg.Rates[namespace]; ok {
		return r, nil
	} else {
		return nil, frlg.Err
	}
}

func newFakeRate() fakeRateLimitGetter {
	return fakeRateLimitGetter{
		Rates: map[string]*sensor.RateLimit{},
	}
}

func TestSensorMutationHook(t *testing.T) {

	t.Parallel()
	frlg := newFakeRate()

	testRate := &sensor.RateLimit{
		Unit:            "Second",
		RequestsPerUnit: int32(2),
	}

	frlg.Rates["test"] = testRate
	frlg.Rates["novalue"] = nil

	rc := ratelimit.NewRateLimitCalculatorOrDie("Second", int32(1))
	defaultRate := rc.Calculate(nil, nil)

	h := NewHandler(&frlg, rc)

	scheme := runtime.NewScheme()
	utilruntime.Must(sensor.AddToScheme(scheme))
	decoder, err := admission.NewDecoder(scheme)
	assert.NoError(t, err)

	h.InjectDecoder(decoder)

	sen := sensor.Sensor{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test",
		},
		Spec: sensor.SensorSpec{},
	}

	tests := []struct {
		description string
		trigger     sensor.Trigger
		ns          string
		expected    float64
	}{
		{
			description: "Trigger w/o k8s target",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "nok8s",
				},
			},
		},
		{
			description: "Trigger w/ namespace allowed k8s target and rate",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "second",
					K8s:  &sensor.StandardK8STrigger{},
				},
				RateLimit: testRate,
			},
		},
		{
			description: "Trigger w/ namespace allowed k8s target and hour rate",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "hour",
					K8s:  &sensor.StandardK8STrigger{},
				},
				RateLimit: &sensor.RateLimit{
					Unit:            "Hour",
					RequestsPerUnit: int32(3600),
				},
			},
		},
		{
			description: "Trigger w/ default k8s target and hour rate",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "default",
					K8s:  &sensor.StandardK8STrigger{},
				},
				RateLimit: &defaultRate,
			},
			ns: "novalue",
		},
		{
			description: "Trigger w/ default k8s target and big rate",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "default",
					K8s:  &sensor.StandardK8STrigger{},
				},
				RateLimit: testRate,
			},
			ns:       "novalue",
			expected: 1,
		},
		{
			description: "Trigger w/ too big k8s target and rate",
			trigger: sensor.Trigger{
				Template: &sensor.TriggerTemplate{
					Name: "mutated",
					K8s:  &sensor.StandardK8STrigger{},
				},
				RateLimit: &sensor.RateLimit{
					Unit:            "Second",
					RequestsPerUnit: int32(100),
				},
			},
			expected: 2,
		},
	}

	for _, test := range tests {

		//set trigger on copy
		csen := sen.DeepCopy()
		csen.Spec.Triggers = []sensor.Trigger{test.trigger}

		if test.ns != "" {
			csen.Namespace = test.ns
		}

		sensorBytes, err := json.Marshal(csen)
		assert.NoError(t, err)

		//make admission request
		ar := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: sensorBytes,
			},
		}

		resp := h.Handle(context.TODO(), admission.Request{AdmissionRequest: ar})

		//test allowed
		assert.True(t, resp.Allowed, "request should be allowed")

		//test patch bytes
		assert.True(t, (len(resp.Patches) > 0) == (test.expected > 0), fmt.Sprintf("%s mutation expected", test.description))
		for _, patch := range resp.Patches {
			assert.Equal(t, "/spec/triggers/0/rateLimit/requestsPerUnit", patch.Path)
			assert.Equal(t, float64(test.expected), patch.Value)
		}
	}
}
