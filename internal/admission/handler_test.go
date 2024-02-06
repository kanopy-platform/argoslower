package admission

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	estest "github.com/kanopy-platform/argoslower/internal/admission/eventsource/testing"
	sensor "github.com/kanopy-platform/argoslower/internal/admission/sensor"
	stest "github.com/kanopy-platform/argoslower/internal/admission/sensor/testing"
	"github.com/kanopy-platform/argoslower/pkg/ratelimit"
	"github.com/stretchr/testify/assert"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	aes "github.com/argoproj/argo-events/pkg/apis/eventsource"
	v1eventsource "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	as "github.com/argoproj/argo-events/pkg/apis/sensor"
	v1sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
)

func TestRoutingHandler(t *testing.T) {

	s := v1sensor.Sensor{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test",
			Name:      "test",
		},
		Spec: v1sensor.SensorSpec{},
	}

	sensorBytes, err := json.Marshal(s)
	assert.NoError(t, err)

	sar := admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: sensorBytes,
		},
	}

	es := v1eventsource.EventSource{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test",
			Name:      "test",
		},
		Spec: v1eventsource.EventSourceSpec{},
	}

	eventSourceBytes, err := json.Marshal(es)
	assert.NoError(t, err)

	esar := admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: eventSourceBytes,
		},
	}

	frlg := stest.NewFakeRate()
	rc := ratelimit.NewRateLimitCalculatorOrDie("Second", int32(80000))

	fmc := &estest.FakeMeshChecker{
		Mesh: true,
	}

	tests := map[string]struct {
		sensor *sensor.Handler
		es     *eventsource.Handler
		sdeny  bool
		esdeny bool
	}{
		"empty":      {sdeny: true, esdeny: true},
		"sensoronly": {sensor: sensor.NewHandler(&frlg, rc), esdeny: true},
		"esonly":     {es: eventsource.NewHandler(fmc), sdeny: true},
		"both":       {sensor: sensor.NewHandler(&frlg, rc), es: eventsource.NewHandler(fmc)},
	}

	for name, test := range tests {

		scheme := runtime.NewScheme()
		decoder, err := admission.NewDecoder(scheme)

		handler := NewRoutingHandler(test.sensor, test.es)
		assert.NoError(t, err)

		err = handler.InjectDecoder(decoder)
		assert.NoError(t, err)

		sck := sar.DeepCopy()
		skind := v1.GroupVersionKind{
			Kind: as.Kind,
		}
		sck.Kind = skind
		sck.RequestKind = &skind

		resp := handler.Handle(context.TODO(), admission.Request{AdmissionRequest: *sck})
		assert.Equal(t, !test.sdeny, resp.Allowed, name)

		sck.RequestKind = nil
		resp = handler.Handle(context.TODO(), admission.Request{AdmissionRequest: *sck})
		assert.Equal(t, !test.sdeny, resp.Allowed, name)

		esck := esar.DeepCopy()
		eskind := v1.GroupVersionKind{
			Kind: aes.Kind,
		}
		esck.Kind = eskind
		esck.RequestKind = &eskind

		resp = handler.Handle(context.TODO(), admission.Request{AdmissionRequest: *esck})
		assert.Equal(t, !test.esdeny, resp.Allowed, name)

		esck.RequestKind = nil
		resp = handler.Handle(context.TODO(), admission.Request{AdmissionRequest: *esck})
		assert.Equal(t, !test.esdeny, resp.Allowed, name)
	}
}
