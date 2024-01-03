package eventsource

import (
	"context"
	"testing"

	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	eslister "github.com/argoproj/argo-events/pkg/client/eventsource/listers/eventsource/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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
	return nil, nil
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

	fakeNSL := &FakeESNamespaceLister{}
	fakeL := &FakeESLister{}
	fakeL.SetNamespaceLister(fakeNSL)

	controller := NewEventSourceIngressController(fakeL)

	_, err := controller.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})

	assert.NoError(t, err)

}
