package eventsource

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	k8serror "k8s.io/apimachinery/pkg/api/errors"

	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	esclient "github.com/argoproj/argo-events/pkg/client/eventsource/clientset/versioned"
)

type EventSourceIngressController struct {
	argoClient  esclient.Interface
	istioClient istioversionedclient.Interface
	config      EventSourceIngressControllerConfig
}

func (e *EventSourceIngressController) Reconcile(ctx context.Context, mgr manager.Manager) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	eventSource := &esv1alpha1.EventSource{}
	if err := e.argoClient.Get(ctx, req.NamespacedName, eventSource); err != nil {
		if k8serror.IsNotFound(err) {
			log.Info("WARNING: eventsource not found", req)
			return ctrl.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("unable to get eventsource %v", req))
		return ctrl.Result{}, err
	}

	if err := e.reconcile(ctx, eventSource.DeepCopy()); err != nil {
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	return ctrl.Result{}, nil

}

func (e *EventSourceIngressController) reconcile(ctx context.Context, es *esv1alpha1.EventSource) error {

	if es == nil {
		return nil
	}

	if es.Webhook != nil {

	}

	return nil

}

func Reconcile(ctx context.Context, esc EventSourceConfig) error {

	return nil
}

type EventSourceIngressControllerConfig struct{}

type EventSourceConfig struct{}
