package istio

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	netv1beta1 "istio.io/api/networking/v1beta1"
	secv1beta1 "istio.io/api/security/v1beta1"
	isv1beta1 "istio.io/api/type/v1beta1"
	isnetv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	issecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	netapplyv1beta1 "istio.io/client-go/pkg/applyconfiguration/networking/v1beta1"
	secapplyv1beta1 "istio.io/client-go/pkg/applyconfiguration/security/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"

	perrs "github.com/kanopy-platform/argoslower/pkg/errors"
	common "github.com/kanopy-platform/argoslower/pkg/ingress"
	v1 "github.com/kanopy-platform/argoslower/pkg/ingress/v1"
)

// IstioClient provides a means of upserting rendered resource from an IstioConfig object
type IstioClient struct {
	client          istioclient.Interface
	gatewaySelector map[string]string
}

func NewClient(cs istioclient.Interface, gs map[string]string) *IstioClient {
	selector := maps.Clone(gs)
	return &IstioClient{
		client:          cs,
		gatewaySelector: selector,
	}
}

func (i *IstioClient) Remove(ctx context.Context, config *v1.EventSourceIngressConfig) error {

	log := log.FromContext(ctx)

	sec := i.client.SecurityV1beta1()
	net := i.client.NetworkingV1beta1()

	if config == nil || config.Eventsource.Name == "" || config.Eventsource.Namespace == "" || config.AdminNamespace == "" || config.Gateway.Namespace == "" {
		return perrs.NewUnretryableError(fmt.Errorf("Empty namespaced name for event source"))
	}

	selector := fmt.Sprintf("%s=%s,%s=%s", common.EventSourceNameString, config.Eventsource.Name, common.EventSourceNamespaceString, config.Eventsource.Namespace)

	listOpts := metav1.ListOptions{
		LabelSelector: selector,
	}

	apErr := sec.AuthorizationPolicies(config.AdminNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	vsErr := net.VirtualServices(config.Gateway.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	if apErr == nil && vsErr == nil {
		return nil
	}

	var errString string
	if apErr != nil && !k8serrors.IsNotFound(apErr) {
		log.V(5).Info("Istio api communication failure: %s", apErr.Error())
		errString = fmt.Sprintf("%s%s", config.Eventsource.String(), apErr.Error())
	}
	if vsErr != nil && !k8serrors.IsNotFound(vsErr) {
		log.V(5).Info("Istio api communication failure: %s", apErr.Error())
		errString = fmt.Sprintf("%s%s", config.Eventsource.String(), vsErr.Error())
	}

	if errString == "" {
		return nil
	}

	return perrs.NewRetryableError(fmt.Errorf("Deleteion errors for selector %s: %s", selector, errString))
}

// UpsertFromConfig creates or updates Virtual Serivces associated with an IstioConfig
// It returns an error for unconfigured IstioConfigs
func (i *IstioClient) upsertFromConfig(config *IstioConfig) (*isnetv1beta1.VirtualService, *issecv1beta1.AuthorizationPolicy, error) {

	sec := i.client.SecurityV1beta1()
	net := i.client.NetworkingV1beta1()

	if !config.IsConfigured() {
		return nil, nil, perrs.NewUnretryableError(errors.New("Unable to configure ingress"))
	}

	vs := config.GetVirtualService()
	vsapply := netapplyv1beta1.VirtualService(vs.Name, vs.Namespace).
		WithLabels(vs.Labels).
		WithAnnotations(vs.Annotations)

	// the netapplyv1beta1.VirtualServiceApplyConfiguration.WithSpec function copies locks by passing a VirtualService Spec by value
	vsapply.Spec = &vs.Spec

	ap := config.GetAuthorizationPolicy()
	apapply := secapplyv1beta1.AuthorizationPolicy(ap.Name, ap.Namespace).
		WithLabels(ap.Labels).
		WithAnnotations(ap.Annotations)
	// the secapplyv1beta1.AuhtorizationPolicyApplyConfiguration.WithSpec function copies locks by passing a VirtualService Spec by value
	apapply.Spec = &ap.Spec

	applyOpts := metav1.ApplyOptions{
		Force:        true,
		FieldManager: "argoslower",
	}

	vso, err := net.VirtualServices(vs.Namespace).Apply(context.TODO(), vsapply, applyOpts)
	if err != nil {
		return vso, nil, err
	}

	apo, err := sec.AuthorizationPolicies(ap.Namespace).Apply(context.TODO(), apapply, applyOpts)
	if err != nil {
		return vso, apo, err
	}

	return vso, apo, err
}

func (i *IstioClient) Configure(ctx context.Context, config *v1.EventSourceIngressConfig) ([]types.NamespacedName, error) {
	log := log.FromContext(ctx)
	out := []types.NamespacedName{}

	if config == nil {
		return out, perrs.NewUnretryableError(fmt.Errorf("Nil config"))
	}

	cidrs, err := config.IPGetter.GetIPs()
	if err != nil {
		log.V(5).Info(fmt.Sprintf("Failed to source IPs for %s: %s", config.Eventsource.String(), err.Error()))
		return out, perrs.NewRetryableError(err)
	}

	if len(cidrs) == 0 {
		e := fmt.Errorf("Failed to source IPs for %s", config.Eventsource.String())
		log.V(1).Info(e.Error())
		return out, perrs.NewRetryableError(e)
	}

	c := NewIstioConfig()
	if e := c.ConfigureAP(config.AdminNamespace, config.BaseURL, config.Eventsource, cidrs, config.Endpoints, i.gatewaySelector); e != nil {
		log.V(5).Info("AuthroizationPolicy generation failure for %s", config.Eventsource.String())
		return out, e
	}
	if e := c.ConfigureVS(config.BaseURL, config.Gateway, config.Service, config.Eventsource, config.Endpoints); e != nil {
		log.V(5).Info("VirtualService generation failure for %s", config.Eventsource.String())
		return out, e
	}

	vs, ap, err := i.upsertFromConfig(c)
	if vs != nil {
		out = append(out, types.NamespacedName{Namespace: vs.Namespace, Name: vs.Name})
	}
	if ap != nil {
		out = append(out, types.NamespacedName{Namespace: vs.Namespace, Name: vs.Name})
	}

	return out, err
}

// IstioConfig contains global configuration for rendering istio VirtualService and AuthorizationPolicy
// resources from a port to endpoint mapping.
type IstioConfig struct {
	ap *issecv1beta1.AuthorizationPolicy
	vs *isnetv1beta1.VirtualService
}

func NewIstioConfig() *IstioConfig {
	return &IstioConfig{}
}

// GetVirtualService returns the current configured or unconfigured virtual service
// if the virtual service is unconfigured it is nil.
func (ic *IstioConfig) GetVirtualService() *isnetv1beta1.VirtualService {
	return ic.vs.DeepCopy()
}

// GetAuthorizationPolicy returns the current configured or unconfigured authorization
// policy if the authorization policy is unconfigured it is nil.
func (ic *IstioConfig) GetAuthorizationPolicy() *issecv1beta1.AuthorizationPolicy {
	return ic.ap.DeepCopy()
}

// IsConfigured returns true if both the ap and vs fields of an IstioConfig
// have been properly Configured
func (ic *IstioConfig) IsConfigured() bool {

	if ic.ap == nil || ic.vs == nil {
		return false
	}

	return true
}

// ConfigureVS populates the IstioConfig.vs field with a VirtualService associated with the IC.
// The VS is associated with the gateway assigned to the IstioConfig and maps paths onto the
// base url in the format baseURL/es.Namespace/es.Name/endpoint/ as a prefix match.
// The virtual service targets the fully qualified internal service host name on the port assigned
// to the endpoint in the port mapping.
func (ic *IstioConfig) ConfigureVS(url string, gw, svc, es types.NamespacedName, endpoints map[string]common.NamedPath) error {
	host := url
	pathPrefix := fmt.Sprintf("/%s/%s", es.Namespace, es.Name)
	svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)

	vs := isnetv1beta1.VirtualService{
		Spec: netv1beta1.VirtualService{
			Hosts:    []string{host},
			Gateways: []string{gw.String()},
		},
	}
	vs.Name = fmt.Sprintf("%s-%s", es.Namespace, es.Name)
	vs.Namespace = gw.Namespace

	vs.Labels = map[string]string{
		common.EventSourceNameString:      es.Name,
		common.EventSourceNamespaceString: es.Namespace,
	}

	routes := make([]*netv1beta1.HTTPRoute, len(endpoints)*2)
	index := 0
	for port, endpoint := range endpoints {
		uport64, err := strconv.ParseUint(port, 10, 32)
		if err != nil {
			continue
		}
		uport := uint32(uport64)

		routes[index] = &netv1beta1.HTTPRoute{
			Name: endpoint.Name,
			DirectResponse: &netv1beta1.HTTPDirectResponse{
				Status: 400,
				Body: &netv1beta1.HTTPBody{
					Specifier: &netv1beta1.HTTPBody_Bytes{
						Bytes: []byte(`{"error":"invalid_request","error_description":"secret too short"}`),
					},
				},
			},
			Match: []*netv1beta1.HTTPMatchRequest{
				&netv1beta1.HTTPMatchRequest{
					Uri: &netv1beta1.StringMatch{
						MatchType: &netv1beta1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("%s%s/", pathPrefix, endpoint.Path),
						},
					},
					Headers: map[string]*netv1beta1.StringMatch{
						"authorization": &netv1beta1.StringMatch{
							MatchType: &netv1beta1.StringMatch_Regex{
								// This regex is lax compared to the spec from
								// https://tools.ietf.org/html/rfc6750#section-2.1
								// but it aligns with the desired length requirements
								// and implementation by argo events
								Regex: `^Bearer\s+\S{0,11}\s*$`,
							},
						},
					},
				},
			},
		}
		index++
		routes[index] = &netv1beta1.HTTPRoute{
			Name: endpoint.Name,
			Route: []*netv1beta1.HTTPRouteDestination{
				&netv1beta1.HTTPRouteDestination{
					Destination: &netv1beta1.Destination{
						Host: svcHost,
						Port: &netv1beta1.PortSelector{
							Number: uport,
						},
					},
				},
			},
			Match: []*netv1beta1.HTTPMatchRequest{
				&netv1beta1.HTTPMatchRequest{
					Uri: &netv1beta1.StringMatch{
						MatchType: &netv1beta1.StringMatch_Prefix{
							Prefix: fmt.Sprintf("%s%s/", pathPrefix, endpoint.Path),
						},
					},
				},
			},
			Rewrite: &netv1beta1.HTTPRewrite{Uri: "/"},
		}
		index++
	}

	if len(routes) == 0 {
		return fmt.Errorf("No viable routes for EventSource %s and Service %s", es.String(), svc.String())
	}

	vs.Spec.Http = routes
	ic.vs = &vs
	return nil
}

// ConfigureAP configures the IstioConfig.ap field with an AuthorizationPolicy base on the inputs.
// The AP will contain a single rule that contains the full IP CIDR list and all paths from the
// endpoint mapping with a glob match. The AP will match the baseURL and baseURL:* hostnames
// per istio host match best practice.
func (ic *IstioConfig) ConfigureAP(adminns, url string, nsn types.NamespacedName, inCIDRs []string, endpoints map[string]common.NamedPath, gws map[string]string) error {

	cidrs := make([]string, len(inCIDRs))
	copy(cidrs, inCIDRs)

	pathPrefix := fmt.Sprintf("/%s/%s", nsn.Namespace, nsn.Name)
	matcher := maps.Clone(gws)

	ap := issecv1beta1.AuthorizationPolicy{
		Spec: secv1beta1.AuthorizationPolicy{
			Selector: &isv1beta1.WorkloadSelector{
				MatchLabels: matcher,
			},
			Action: secv1beta1.AuthorizationPolicy_DENY,
		},
	}
	ap.Name = fmt.Sprintf("%s-%s", nsn.Namespace, nsn.Name)
	ap.Namespace = adminns
	ap.Labels = map[string]string{
		"eventsource-name":      nsn.Name,
		"eventsource-namespace": nsn.Namespace,
	}

	paths := make([]string, len(endpoints))
	index := 0
	for _, path := range endpoints {
		paths[index] = pathPrefix + path.Path + "/*"
		index++
	}
	if len(paths) == 0 {
		return fmt.Errorf("EventSource %s has no valid paths for its service configuration.", nsn.String())
	}

	source := &secv1beta1.Source{}
	if ap.Spec.Action == secv1beta1.AuthorizationPolicy_DENY {
		source.NotIpBlocks = cidrs
	} else {
		source.IpBlocks = cidrs
	}

	rule := &secv1beta1.Rule{
		From: []*secv1beta1.Rule_From{
			&secv1beta1.Rule_From{
				Source: source,
			},
		},
		To: []*secv1beta1.Rule_To{
			&secv1beta1.Rule_To{
				Operation: &secv1beta1.Operation{
					Hosts: []string{
						url,
						fmt.Sprintf("%s:*", url),
					},
					Paths: paths,
				},
			},
		},
	}

	ap.Spec.Rules = append(ap.Spec.Rules, rule)

	ic.ap = &ap
	return nil
}
