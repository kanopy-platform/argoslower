package eventsource_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	estest "github.com/kanopy-platform/argoslower/internal/admission/eventsource/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	common "github.com/argoproj/argo-events/pkg/apis/common"
	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
)

func TestEventSourceHandler(t *testing.T) {

	t.Parallel()
	fmc := &estest.FakeMeshChecker{
		Mesh: true,
	}

	knownSources := map[string]bool{
		"github": true,
	}

	handler := eventsource.NewHandler(fmc, knownSources)
	scheme := runtime.NewScheme()
	utilruntime.Must(esv1alpha1.AddToScheme(scheme))
	decoder := admission.NewDecoder(scheme)
	err := handler.InjectDecoder(decoder)
	assert.NoError(t, err)

	f := false

	tests := []struct {
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
						eventsource.DefaultAnnotationKey: "github",
					},
				},
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{
						"ghs": esv1alpha1.GithubEventSource{
							WebhookSecret: &corev1.SecretKeySelector{},
						},
					},
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
						eventsource.DefaultAnnotationKey: "github",
					},
				},
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{
						"ghs": esv1alpha1.GithubEventSource{
							WebhookSecret: &corev1.SecretKeySelector{},
						},
					},
				},
			},
		},
		{
			name: "Namespace Opted Out",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "github",
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
						eventsource.DefaultAnnotationKey: "github",
					},
				},
			},
			nsErr: errors.New("test error"),
		},
		{
			name: "Unknown Source",
			es: esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventsource.DefaultAnnotationKey: "unknown-source",
					},
				},
			},
			err: true,
		},
	}

	for _, test := range tests {
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
			assert.Equal(t, test.nsErr.Error(), resp.AdmissionResponse.Result.Message, test.name)
			continue
		}

		_, ok := test.es.ObjectMeta.Annotations[test.key]
		if !ok {
			assert.True(t, resp.AdmissionResponse.Allowed, test.name)
			assert.Equal(t, 0, len(resp.Patches), test.name)
			continue
		}

		if test.err {
			assert.False(t, resp.AdmissionResponse.Allowed, test.name)
			assert.Contains(t, resp.AdmissionResponse.Result.Reason, "Forbidden", test.name)
			assert.Contains(t, resp.AdmissionResponse.Result.Message, "Unknown webhook source", test.name)
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

func TestValidateEventSource(t *testing.T) {

	tests := map[string]struct {
		spec *esv1alpha1.EventSource
		err  bool
	}{

		"no sources": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "empty",
					Namespace: "testing",
				},
			},
			err: true,
		},
		"github no secret": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "nosecret",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{},
				},
			},
			err: true,
		},
		"github secret": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "valid",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{
						"ghs": esv1alpha1.GithubEventSource{
							WebhookSecret: &corev1.SecretKeySelector{},
						},
					},
				},
			},
		},
		"github mixed": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "nosecret",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{
						"ghs": esv1alpha1.GithubEventSource{
							WebhookSecret: &corev1.SecretKeySelector{},
						},
						"nos": esv1alpha1.GithubEventSource{},
					},
				},
			},
			err: true,
		},
		"webhook no secret": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "nosecret",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Webhook: map[string]esv1alpha1.WebhookEventSource{
						"nos": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{},
						},
					},
				},
			},
			err: true,
		},
		"webhook valid": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "valid",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Webhook: map[string]esv1alpha1.WebhookEventSource{
						"ws": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{
								AuthSecret: &corev1.SecretKeySelector{},
							},
						},
					},
				},
			},
		},
		"webhook mixed": {
			spec: &esv1alpha1.EventSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "nosecret",
					Namespace: "testing",
				},
				Spec: esv1alpha1.EventSourceSpec{
					Webhook: map[string]esv1alpha1.WebhookEventSource{
						"ws": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{
								AuthSecret: &corev1.SecretKeySelector{},
							},
						},
						"nos": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{},
						},
					},
				},
			},
			err: true,
		},
	}

	for name, test := range tests {
		e := eventsource.ValidateEventSource(test.spec)

		if test.err {
			assert.Error(t, e, name)
		} else {
			assert.NoError(t, e, name)
		}
	}

}
