package eventsource

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eshandler "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	ingresscommon "github.com/kanopy-platform/argoslower/pkg/ingress"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"

	eventscommon "github.com/argoproj/argo-events/common"
	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	eslister "github.com/argoproj/argo-events/pkg/client/eventsource/listers/eventsource/v1alpha1"

	"k8s.io/apimachinery/pkg/labels"
)

type EventSourceIngressController struct {
	esLister      eslister.EventSourceLister
	serviceLister corev1lister.ServiceLister
	config        EventSourceIngressControllerConfig
}

type EventSourceIngressControllerConfig struct {
	gateway        types.NamespacedName
	baseURL        string
	adminNamespace string
}

func NewEventSourceIngressController(esl eslister.EventSourceLister, svcl corev1lister.ServiceLister, config EventSourceIngressControllerConfig) *EventSourceIngressController {
	return &EventSourceIngressController{
		esLister:      esl,
		serviceLister: svcl,
		config:        config,
	}
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
	log.V(5).Info("Starting reconciliation for %s/%s", nsn.Namespace, nsn.Name)

	esiConfig := EventSourceIngressConfig{
		es:             nsn,
		gateway:        e.config.gateway,
		adminNamespace: e.config.adminNamespace,
	}
	if es == nil {
		//TODO: implement garbage collection we should be able to use the NamespacedName to associated vanity resources to the namespace/eventsource
		// event source svc labels
		//   labels:
		//      controller: eventsource-controller
		//      eventsource-name: demo-day
		//      owner-name: demo-day
		// look up and delete virtual service
		// look up and delete authorization policy
		//
		// look up and delete virtual service delegate
		return nil
	}

	hookType, ok := es.Annotations[eshandler.DefaultAnnotationKey]
	if !ok {
		//TODO: should this get filtered at admission time for a list of supported values?
		return nil
	}

	selector, err := labels.ValidatedSelectorFromSet(
		labels.Set(map[string]string{
			eventscommon.LabelEventSourceName: nsn.Name,
		}),
	)
	if err != nil {
		return err
	}

	svcl, err := e.serviceLister.Services(nsn.Namespace).List(selector)
	if err != nil {
		return err
	}

	if len(svcl) != 1 {
		return fmt.Errorf("Cannot Select a service for event source %s/%s. Want 1 received: %d", nsn.Namespace, nsn.Name, len(svcl))
	}

	svc := svcl[0]
	if svc == nil {
		return fmt.Errorf("Expected a Service resource and got: %v", svc)
	}

	// Populate the Service to EventSource lookup map
	esiConfig.endpoints = ServiceToPortMapping(svc, es)
	if len(esiConfig.endpoints) == 0 {
		//TODO: we might want to emit an event here for a misconfigured eventsource
		// the port on the webhook configuration doesn't appear on the service definition
		// it is excluded because it isn't routable
		return nil
	}

	switch hookType {
	case "github":
		//TODO: set github ip getter

	case "jira":
		//TODO: set jira ip getter

	default:
		//TODO: Log event for unsupported webhook type
		return nil

	}
	return nil
}

// EventSourceIngressConfig provides the information needed for rendering
// ingress resources mapped to the service of an argo event source.
// it is ingress provider agnostic.
type EventSourceIngressConfig struct {
	//	ipg            IPGetter
	es             types.NamespacedName
	endpoints      map[string]ingresscommon.NamedPath
	adminNamespace string
	//	baseURL        string
	gateway types.NamespacedName
}

// ServiceToPortMapping - receives a Service and argo EventSource and returns a validated port lookup map of
// NamedPaths. The lookup map allows mapping a target port to a desired path. It is generic for any eventsource
// but only provides data for supported event source types, github and webhook currently.
func ServiceToPortMapping(svc *corev1.Service, es *esv1alpha1.EventSource) (out map[string]ingresscommon.NamedPath) {
	out = map[string]ingresscommon.NamedPath{}
	//Only github and webhook eventsources are supported for self-service webhooks currently.
	//if neither of those are configured don't offer any ports
	if svc == nil || es == nil {
		return out
	}

	for _, svcport := range svc.Spec.Ports {
		out[fmt.Sprintf("%d", svcport.Port)] = ingresscommon.NamedPath{}
	}
	for esn, spec := range es.Spec.Webhook {
		np, ok := out[spec.Port]
		if !ok {
			continue
		}
		np.Name = esn
		np.Path = spec.Endpoint
		out[spec.Port] = np
	}

	for esn, spec := range es.Spec.Github {
		if spec.Webhook == nil {
			continue
		}
		np, ok := out[spec.Webhook.Port]
		if !ok {
			continue
		}
		np.Name = esn
		np.Path = spec.Webhook.Endpoint
		out[spec.Webhook.Port] = np
	}

	return out
}

// IPGetter defines an interface for ip address providers to get source CIDRs
type IPGetter interface {
	GetIPs() ([]string, error)
}
