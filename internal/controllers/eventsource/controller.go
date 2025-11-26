package eventsource

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eshandler "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	perrs "github.com/kanopy-platform/argoslower/pkg/errors"
	ingresscommon "github.com/kanopy-platform/argoslower/pkg/ingress"
	v1 "github.com/kanopy-platform/argoslower/pkg/ingress/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"

	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	eslister "github.com/argoproj/argo-events/pkg/client/listers/events/v1alpha1"

	"k8s.io/apimachinery/pkg/labels"
)

type EventSourceIngressController struct {
	esLister      eslister.EventSourceLister
	serviceLister corev1lister.ServiceLister
	igc           v1.IngressConfigurator
	config        EventSourceIngressControllerConfig
}

type EventSourceIngressControllerConfig struct {
	Gateway        types.NamespacedName
	BaseURL        string
	AdminNamespace string
	ipGetters      map[string]v1.IPGetter
}

func NewEventSourceIngressControllerConfig() EventSourceIngressControllerConfig {
	return EventSourceIngressControllerConfig{
		ipGetters: map[string]v1.IPGetter{},
	}
}
func (c *EventSourceIngressControllerConfig) SetIPGetter(name string, getter v1.IPGetter) {
	if c.ipGetters == nil {
		c.ipGetters = map[string]v1.IPGetter{}
	}

	c.ipGetters[name] = getter
}

func (c *EventSourceIngressControllerConfig) GetKnownSources() map[string]bool {
	knownSources := make(map[string]bool)
	for source := range c.ipGetters {
		knownSources[source] = true
	}
	return knownSources
}

func NewEventSourceIngressController(esl eslister.EventSourceLister, svcl corev1lister.ServiceLister, config EventSourceIngressControllerConfig, igc v1.IngressConfigurator) *EventSourceIngressController {
	return &EventSourceIngressController{
		esLister:      esl,
		serviceLister: svcl,
		config:        config,
		igc:           igc,
	}
}

func (e *EventSourceIngressController) SetIPGetter(name string, getter v1.IPGetter) {
	if e.config.ipGetters == nil {
		e.config.ipGetters = map[string]v1.IPGetter{}
	}

	e.config.ipGetters[name] = getter
}

func (e *EventSourceIngressController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	eventSource, err := e.esLister.EventSources(req.Namespace).Get(req.Name)
	if err != nil && !k8serror.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("unable to get eventsource %v", req))
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	if err = e.reconcile(ctx, eventSource.DeepCopy(), req.NamespacedName); err != nil {
		var retry bool
		rerr, ok := err.(*perrs.RetryableError)
		if ok {
			retry = rerr.IsRetryable()
		}
		return ctrl.Result{
			Requeue: retry,
		}, err
	}

	return ctrl.Result{}, nil

}

func (e *EventSourceIngressController) reconcile(ctx context.Context, es *esv1alpha1.EventSource, nsn types.NamespacedName) error {
	log := log.FromContext(ctx)
	log.V(5).Info("Starting reconciliation for %s/%s", nsn.Namespace, nsn.Name)

	esiConfig := v1.EventSourceIngressConfig{
		Eventsource:    nsn,
		Gateway:        e.config.Gateway,
		AdminNamespace: e.config.AdminNamespace,
		BaseURL:        e.config.BaseURL,
	}
	if es == nil {
		return e.igc.Remove(ctx, &esiConfig)
	}

	hookType, ok := es.Annotations[eshandler.DefaultAnnotationKey]
	if !ok {
		//TODO: should this get filtered at admission time for a list of supported values?
		return nil
	}

	selector, err := labels.ValidatedSelectorFromSet(
		labels.Set(map[string]string{
			esv1alpha1.LabelEventSourceName: nsn.Name,
		}),
	)
	if err != nil {
		return perrs.NewUnretryableError(err)
	}

	log.V(5).Info("Sourcing service for eventsource  %s/%s", nsn.Namespace, nsn.Name)
	svcl, err := e.serviceLister.Services(nsn.Namespace).List(selector)
	if err != nil {
		return perrs.NewRetryableError(err)
	}

	if len(svcl) != 1 {
		return perrs.NewRetryableError(fmt.Errorf("cannot select a service for event source %s/%s. Want 1 received: %d", nsn.Namespace, nsn.Name, len(svcl)))
	}

	svc := svcl[0]
	if svc == nil {
		//This might not be retryable if the api is returning a nil service but we will requeue for now
		return perrs.NewRetryableError(fmt.Errorf("expected a Service resource and got: %v", svc))
	}

	esiConfig.Service = types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      svc.Name,
	}

	// Populate the Service to EventSource lookup map
	esiConfig.Endpoints = ServiceToPortMapping(svc, es)
	if len(esiConfig.Endpoints) == 0 {
		//TODO: we might want to emit an event here for a misconfigured eventsource
		// the port on the webhook configuration doesn't appear on the service definition
		// it is excluded because it isn't routable
		msg := fmt.Sprintf("Unabled to map eventsource  %s.service to endpoints.", nsn.String())
		return perrs.NewUnretryableError(errors.New(msg))
	}

	ipGetter, ok := e.config.ipGetters[hookType]
	if !ok {
		msg := fmt.Sprintf("EventSource %s Hook type: %s, not supported", nsn.String(), hookType)
		return perrs.NewUnretryableError(errors.New(msg))
	}

	esiConfig.IPGetter = ipGetter
	names, err := e.igc.Configure(ctx, &esiConfig)
	log.V(5).Info("Created resources %s", names)
	return err
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
