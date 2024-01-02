package eventsource

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eshandler "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	k8serror "k8s.io/apimachinery/pkg/api/errors"

	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	eslister "github.com/argoproj/argo-events/pkg/client/eventsource/listers/eventsource/v1alpha1"
)

type EventSourceIngressController struct {
	esLister eslister.EventSourceLister
	//	config    EventSourceIngressControllerConfig
}

func (e *EventSourceIngressController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	eventSource, err := e.esLister.EventSources(req.NamespacedName.Namespace).Get(req.NamespacedName.Name)
	if err != nil {
		if k8serror.IsNotFound(err) {
			log.Info("WARNING: eventsource not found", req)
			return ctrl.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("unable to get eventsource %v", req))
		return ctrl.Result{}, err
	}

	//If the event source isn't annotated it can be ignored
	if _, ok := eventSource.Annotations[eshandler.DefaultAnnotationKey]; !ok {
		return ctrl.Result{}, nil

	}

	if err := e.reconcile(ctx, eventSource.DeepCopy()); err != nil {
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	return ctrl.Result{}, nil

}

func (e *EventSourceIngressController) reconcile(ctx context.Context, es *esv1alpha1.EventSource) error {
	log := log.FromContext(ctx)
	if es == nil {
		return nil
	}

	hookType := es.Annotations[eshandler.DefaultAnnotationKey]

	switch hookType {
	case "github":
		log.Info("populate ingress config for github")

	case "jira":
		log.Info("populate ingress config for a generic webhook")

		if es.Spec.Webhook != nil {
			return nil
		}

	default:

	}

	return nil

}

// IngressConfiguratorConfig proto
// ipcidrs []string sourced from ip listers
// selectors map[string]string sourced from eventsource
// namespace
// name

func Reconcile(ctx context.Context, esc EventSourceConfig) error {

	return nil
}

type EventSourceIngressControllerConfig struct{}

type EventSourceConfig struct{}
