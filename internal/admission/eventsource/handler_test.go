package eventsource_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	common "github.com/argoproj/argo-events/pkg/apis/common"
	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
)

type FakeMeshChecker struct {
	Mesh bool
	Err  error
}

func (m *FakeMeshChecker) OnMesh(ns string) (bool, error) {
	return m.Mesh, m.Err
}

func TestEventSourceHandler(t *testing.T) {

	t.Parallel()
	fmc := &FakeMeshChecker{
		Mesh: true,
	}

	handler := eventsource.NewHandler(fmc)
	scheme := runtime.NewScheme()
	utilruntime.Must(esv1alpha1.AddToScheme(scheme))
	decoder, err := admission.NewDecoder(scheme)
	assert.NoError(t, err)
	err = handler.InjectDecoder(decoder)
	assert.NoError(t, err)

	f := false
	for _, test := range []struct {
		name     string
		es       esv1alpha1.EventSource
		key      string
		err      bool
		nsOnMesh *bool
		nsErr    error
	}{
		{
			name: "No Annotation",
		},
		{
			name: "Opted In, labels",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "true",
					},
				},
				Spec: esv1alpha1.EventSourceSpec{
					Template: &esv1alpha1.Template{
						Metadata: &common.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
		},
		{
			name: "Opted In, no labels",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "false",
					},
				},
			},
		},
		{
			name: "Namespace Opted Out",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "true",
					},
				},
			},
			nsOnMesh: &f,
		},
		{
			name: "Namespace Error",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "true",
					},
				},
			},
			nsErr: errors.New("test error"),
		},
	} {
		if test.key == "" {
			test.key = eventsource.DefaultAnnotationKey
		}

		if test.nsOnMesh != nil {
			fmc.Mesh = false
		}

		fmc.Err = test.nsErr

		esb, err := json.Marshal(test.es)
		require.NoError(t, err)

		ar := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: esb,
			},
		}

		resp := handler.Handle(context.TODO(), admission.Request{AdmissionRequest: ar})
		fmc.Mesh = true
		fmc.Err = nil

		if test.nsErr != nil {
			assert.False(t, resp.AdmissionResponse.Allowed, test.name)
			assert.True(t, resp.AdmissionResponse.Result.Message == test.nsErr.Error(), test.name)
			continue
		}

		_, ok := test.es.ObjectMeta.Annotations[test.key]
		if !ok {
			assert.True(t, resp.AdmissionResponse.Allowed, test.name)
			assert.Equal(t, 0, len(resp.Patches), test.name)
			continue
		}

		if test.nsOnMesh != nil {
			assert.False(t, resp.AdmissionResponse.Allowed, test.name)
			continue
		}

		assert.True(t, resp.AdmissionResponse.Allowed, test.name)
		assert.Equal(t, 1, len(resp.Patches), test.name)

	}
}
