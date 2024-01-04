package eventsource

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eshandler "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
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

	esiConfig.endpoints = ServiceToPortMapping(svc, es)
	log.Info(fmt.Sprintf("%v", esiConfig.endpoints))
	fmt.Println(esiConfig.endpoints)
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

type NamedPath struct {
	name string
	path string
}

// IngressConfiguratorConfig proto
// ipcidrs []string sourced from ip listers
// selectors map[string]string sourced from eventsource
// gateway name/namespce
// namespace
// name
type EventSourceIngressConfig struct {
	ipg            IPGetter
	es             types.NamespacedName
	endpoints      map[string]NamedPath
	adminNamespace string
	baseURL        string
	gateway        types.NamespacedName
}

/*
func (e *EventSourceIngressConfig) RenderResources(client *v1istio.Client) (*isnetv1beta1.VirtualService, *issecv1beta1.AuthorizationPolicy) {

	// VirtualService

	vs := &isnetv1beta1.VirtualService{}
	vs.Name = fmt.Sprintf("%s-%s", es.Namespace, es.Name)
	vs.Namespace = e.adminNamespace

	// AuthorizationPolicy

	ap := &issecv1beta1.AuthorizationPolicy{}
	ap.Name = fmt.Sprintf("%s-%s", es.Namespace, es.Name)
	ap.Namespace = e.adminNamespace
}
*/
func ServiceToPortMapping(svc *corev1.Service, es *esv1alpha1.EventSource) (out map[string]NamedPath) {
	out = map[string]NamedPath{}
	//Only github and webhook eventsources are supported for self-service webhooks currently.
	//if neither of those are configured don't offer any ports
	if svc == nil || es == nil || (es.Spec.Webhook == nil && es.Spec.Github == nil) {
		return out
	}

	for _, svcport := range svc.Spec.Ports {
		out[string(svcport.Port)] = NamedPath{}
	}
	for esn, spec := range es.Spec.Webhook {
		np, ok := out[spec.Port]
		if !ok {
			continue
		}
		np.name = esn
		np.path = spec.Endpoint
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
		np.name = esn
		np.path = spec.Webhook.Endpoint
		out[spec.Webhook.Port] = np
	}

	return out
}

type IPGetter interface {
	GetIPs() ([]string, error)
}
