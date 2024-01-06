package eventsource

import (
	"context"
	"testing"

	ingresscommon "github.com/kanopy-platform/argoslower/pkg/ingress"

	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	eslister "github.com/argoproj/argo-events/pkg/client/eventsource/listers/eventsource/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type FakeESLister struct {
	lister eslister.EventSourceNamespaceLister
	eslister.EventSourceListerExpansion
}

func (f *FakeESLister) SetNamespaceLister(esl eslister.EventSourceNamespaceLister) {
	f.lister = esl
}

func (f *FakeESLister) List(selector labels.Selector) (ret []*esv1alpha1.EventSource, err error) {
	return []*esv1alpha1.EventSource{}, nil
}

func (f *FakeESLister) EventSources(ns string) eslister.EventSourceNamespaceLister {
	if f == nil {
		return nil
	}

	return f.lister
}

type FakeESNamespaceLister struct {
	eventSources []*esv1alpha1.EventSource
	errs         []error
	eslister.EventSourceNamespaceListerExpansion
}

func (f *FakeESNamespaceLister) List(selector labels.Selector) (ret []*esv1alpha1.EventSource, err error) {
	return []*esv1alpha1.EventSource{}, nil
}

func (f *FakeESNamespaceLister) Get(name string) (*esv1alpha1.EventSource, error) {
	return f.Next()
}

func (f *FakeESNamespaceLister) AppendES(es *esv1alpha1.EventSource) {
	f.eventSources = append(f.eventSources, es)
}

func (f *FakeESNamespaceLister) AppendError(err error) {
	f.errs = append(f.errs, err)

}

func (f *FakeESNamespaceLister) Next() (*esv1alpha1.EventSource, error) {
	var out *esv1alpha1.EventSource
	var err error
	if len(f.eventSources) > 0 {
		out = f.eventSources[0]
		f.eventSources = f.eventSources[1:]
	}

	if len(f.errs) > 0 {
		err = f.errs[0]
		f.errs = f.errs[1:]
	}

	return out, err

}

func TestReconcile(t *testing.T) {

	fakeNESL := &FakeESNamespaceLister{}
	fakeESL := &FakeESLister{}
	fakeESL.SetNamespaceLister(fakeNESL)

	fakeNSL := &FakeServiceNamespaceLister{}
	fakeSL := &FakeServiceLister{}
	fakeSL.SetNamespaceLister(fakeNSL)

	config := EventSourceIngressControllerConfig{
		gateway: types.NamespacedName{
			Name:      "gateway",
			Namespace: "gatewaynnamespace",
		},
		baseURL:        "webhooks.example.com",
		adminNamespace: "adminnamespace",
	}

	controller := NewEventSourceIngressController(fakeESL, fakeSL, config)

	_, err := controller.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})

	assert.NoError(t, err)

	es := &esv1alpha1.EventSource{}
	es.Namespace = "foo"
	es.Name = "bar"
	fakeNESL.AppendES(es)

	svc := &corev1.Service{}
	svc.Name = "bar-eventsource-svc"
	svc.Namespace = "foo"
	fakeNSL.AppendService(svc)

	_, err = controller.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
	assert.NoError(t, err)

	es.Annotations = map[string]string{"v1alpha1.argoslower.kanopy-platform/known-source": "jira"}
	fakeNESL.AppendES(es)

	_, err = controller.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
	assert.NoError(t, err)
}

type FakeServiceLister struct {
	svclister corev1lister.ServiceNamespaceLister
	corev1lister.ServiceListerExpansion
}

func (fsl *FakeServiceLister) SetNamespaceLister(svcnl corev1lister.ServiceNamespaceLister) {
	fsl.svclister = svcnl
}

func (fsl *FakeServiceLister) List(selector labels.Selector) (ret []*corev1.Service, err error) {
	return []*corev1.Service{}, nil
}

func (fsl *FakeServiceLister) Services(ns string) corev1lister.ServiceNamespaceLister {
	if fsl == nil {
		return nil
	}

	return fsl.svclister
}

type FakeServiceNamespaceLister struct {
	services []*corev1.Service
	errs     []error
	corev1lister.ServiceNamespaceListerExpansion
}

func (f *FakeServiceNamespaceLister) AppendService(svc *corev1.Service) {
	f.services = append(f.services, svc)
}

func (f *FakeServiceNamespaceLister) AppendError(err error) {
	f.errs = append(f.errs, err)
}

func (f *FakeServiceNamespaceLister) List(selector labels.Selector) ([]*corev1.Service, error) {
	var e error
	if f == nil {
		return []*corev1.Service{}, e
	}
	if len(f.errs) > 0 {
		e = f.errs[0]
		f.errs = f.errs[1:]
	}
	return f.services, e
}

func (f *FakeServiceNamespaceLister) Get(name string) (*corev1.Service, error) {
	return f.Next()
}

func (f *FakeServiceNamespaceLister) Next() (*corev1.Service, error) {
	var out *corev1.Service
	var err error
	if f == nil {
		return out, err
	}

	if len(f.services) > 0 {
		out = f.services[0]
		f.services = f.services[1:]
	}

	if len(f.errs) > 0 {
		err = f.errs[0]
		f.errs = f.errs[1:]
	}

	return out, err
}

func TestServiceToPortMapping(t *testing.T) {
	tests := []struct {
		name     string
		svc      *corev1.Service
		es       *esv1alpha1.EventSource
		expected map[string]ingresscommon.NamedPath
	}{
		{
			name:     "empty",
			svc:      &corev1.Service{},
			es:       &esv1alpha1.EventSource{},
			expected: map[string]ingresscommon.NamedPath{},
		},
		{
			name: "webhook",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Port: int32(12345),
						},
					},
				},
			},
			es: &esv1alpha1.EventSource{
				Spec: esv1alpha1.EventSourceSpec{
					Webhook: map[string]esv1alpha1.WebhookEventSource{
						"thing": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{
								Endpoint: "/path",
								Port:     "12345",
							},
						},
					},
				},
			},
			expected: map[string]ingresscommon.NamedPath{
				"12345": ingresscommon.NamedPath{
					Name: "thing",
					Path: "/path",
				},
			},
		},
		{
			name: "multiple webhooks",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Port: int32(12345),
						},
						corev1.ServicePort{
							Port: int32(54321),
						},
					},
				},
			},
			es: &esv1alpha1.EventSource{
				Spec: esv1alpha1.EventSourceSpec{
					Webhook: map[string]esv1alpha1.WebhookEventSource{
						"thingOne": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{
								Endpoint: "/path",
								Port:     "12345",
							},
						},
						"thingTwo": esv1alpha1.WebhookEventSource{
							WebhookContext: esv1alpha1.WebhookContext{
								Endpoint: "/path",
								Port:     "54321",
							},
						},
					},
				},
			},
			expected: map[string]ingresscommon.NamedPath{
				"12345": ingresscommon.NamedPath{
					Name: "thingOne",
					Path: "/path",
				},
				"54321": ingresscommon.NamedPath{
					Name: "thingTwo",
					Path: "/path",
				},
			},
		},
		{
			name: "github",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Port: int32(12345),
						},
					},
				},
			},
			es: &esv1alpha1.EventSource{
				Spec: esv1alpha1.EventSourceSpec{
					Github: map[string]esv1alpha1.GithubEventSource{
						"github": esv1alpha1.GithubEventSource{
							Webhook: &esv1alpha1.WebhookContext{
								Endpoint: "/path",
								Port:     "12345",
							},
						},
					},
				},
			},
			expected: map[string]ingresscommon.NamedPath{
				"12345": ingresscommon.NamedPath{
					Name: "github",
					Path: "/path",
				},
			},
		},
	}

	for _, test := range tests {
		out := ServiceToPortMapping(test.svc, test.es)
		assert.Equal(t, len(test.expected), len(out), test.name)
		for k, v := range test.expected {
			val, ok := out[k]
			assert.True(t, ok, test.name)
			assert.Equal(t, v.Name, val.Name, test.name)
			assert.Equal(t, v.Path, val.Path, test.name)
		}

	}

}
