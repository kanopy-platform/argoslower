package eventsource

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eshandler "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

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
	if err != nil && !k8serror.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("unable to get eventsource %v", req))
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	if err = e.reconcile(ctx, eventSource.DeepCopy(), req.NamespacedName); err != nil {
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	return ctrl.Result{}, nil

}

func (e *EventSourceIngressController) reconcile(ctx context.Context, es *esv1alpha1.EventSource, nsn types.NamespacedName) error {
	log := log.FromContext(ctx)
	if es == nil {
		//TODO: implement garbage collection we should be able to use the NamespacedName to associated vanity resources to the namespace/eventsource
		// event source svc labels
		//   labels:
		//      controller: eventsource-controller
		//      eventsource-name: demo-day
		//      owner-name: demo-day
		return nil
	}

	hookType, ok := es.Annotations[eshandler.DefaultAnnotationKey]
	if !ok {
		return nil
	}

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
