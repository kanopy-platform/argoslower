package istio

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	netv1beta1 "istio.io/api/networking/v1beta1"
	secv1beta1 "istio.io/api/security/v1beta1"
	isv1beta1 "istio.io/api/type/v1beta1"
	isnetv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	issecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	netapplyv1beta1 "istio.io/client-go/pkg/applyconfiguration/networking/v1beta1"
	secapplyv1beta1 "istio.io/client-go/pkg/applyconfiguration/security/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"

	esc "github.com/kanopy-platform/argoslower/internal/controllers/eventsource"
	perrs "github.com/kanopy-platform/argoslower/pkg/errors"
	common "github.com/kanopy-platform/argoslower/pkg/ingress"
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

// UpsertFromConfig creates or updates Virtual Serivces associated with an IstioConfig
// It returns an error for unconfigured IstioConfigs
func (i *IstioClient) UpsertFromConfig(config *IstioConfig) (*isnetv1beta1.VirtualService, *issecv1beta1.AuthorizationPolicy, error) {

	//TODO: code to create or apply VirtualService and AuthorizationPolicies
	sec := i.client.SecurityV1beta1()
	net := i.client.NetworkingV1beta1()

	if !config.IsConfigured() {
		return nil, nil, perrs.NewUnretryableError(errors.New("Unable to configure ingress"))
	}

	vs := config.GetVirtualService()
	vsapply := netapplyv1beta1.VirtualService(vs.Name, vs.Namespace).
		WithLabels(vs.Labels).
		WithAnnotations(vs.Annotations).
		WithSpec(vs.Spec)

	ap := config.GetAuthorizationPolicy()
	apapply := secapplyv1beta1.AuthorizationPolicy(ap.Name, ap.Namespace).
		WithLabels(ap.Labels).
		WithAnnotations(ap.Annotations).
		WithSpec(ap.Spec)

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

func (i *IstioClient) Configure(config *esc.EventSourceIngressConfig) ([]types.NamespacedName, error) {
	cidrs, err := config.Ipg.GetIPs()
	out := []types.NamespacedName{}
	if err != nil {
		return out, perrs.NewUnretryableError(err)
	}

	c := NewIstioConfig()
	if e := c.ConfigureAP(config.AdminNamespace, config.BaseURL, config.Es, cidrs, config.Endpoints, i.gatewaySelector); e != nil {
		return out, e
	}
	if e := c.ConfigureVS(config.BaseURL, config.Gateway, config.Service, config.Es, config.Endpoints); e != nil {
		return out, e
	}

	vs, ap, err := i.UpsertFromConfig(c)
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
		"eventsource-name":      es.Name,
		"eventsource-namespace": es.Namespace,
	}

	routes := make([]*netv1beta1.HTTPRoute, len(endpoints))
	index := 0
	for port, endpoint := range endpoints {
		uport64, err := strconv.ParseUint(port, 10, 32)
		if err != nil {
			continue
		}
		uport := uint32(uport64)

		route := netv1beta1.HTTPRoute{
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

		routes[index] = &route
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
