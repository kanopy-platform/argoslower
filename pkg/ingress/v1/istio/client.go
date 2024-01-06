package istio

import (
	"fmt"
	"maps"
	"strconv"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	isnetlister "istio.io/client-go/pkg/listers/networking/v1beta1"
	isseclister "istio.io/client-go/pkg/listers/security/v1beta1"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	netv1beta1 "istio.io/api/networking/v1beta1"
	secv1beta1 "istio.io/api/security/v1beta1"
	isv1beta1 "istio.io/api/type/v1beta1"
	isnetv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	issecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"

	common "github.com/kanopy-platform/argoslower/pkg/ingress"
)

type IstioClient struct {
	client     istioclient.Interface
	vsl        isnetlister.VirtualServiceLister
	apl        isseclister.AuthorizationPolicyLister
	gateway    types.NamespacedName
	gwSelector map[string]string
	baseURL    string
}

func NewClient(cs istioclient.Interface, vsl isnetlister.VirtualServiceLister, apl isseclister.AuthorizationPolicyLister, gw types.NamespacedName, gws map[string]string, baseURL string) *IstioClient {
	return &IstioClient{
		client:     cs,
		vsl:        vsl,
		apl:        apl,
		gateway:    gw,
		gwSelector: gws,
		baseURL:    baseURL,
	}
}

func (i *IstioClient) NewConfig(nsn types.NamespacedName) (*IstioConfig, error) {
	out := &IstioConfig{}
	decoratedName := fmt.Sprintf("%s-%s", i.gateway.Namespace, nsn.Name)

	ap, err := i.apl.AuthorizationPolicies(i.gateway.Namespace).Get(decoratedName)
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return out, err
		}
		ap = &issecv1beta1.AuthorizationPolicy{}
		ap.Name = decoratedName
		ap.Namespace = i.gateway.Namespace
	}

	vs, err := i.vsl.VirtualServices(i.gateway.Namespace).Get(fmt.Sprintf("%s-%s", i.gateway.Namespace, nsn.Name))
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return out, err
		}
		vs = &isnetv1beta1.VirtualService{}
		vs.Name = decoratedName
		vs.Namespace = i.gateway.Namespace
	}

	out = &IstioConfig{
		ap: ap.DeepCopy(),
		vs: vs.DeepCopy(),
	}

	return out, nil
}

func (i *IstioClient) SetMetadata() {

}

func (i *IstioClient) UpsertFromConfig(cfg *IstioConfig) error {

	return nil
}

type IstioConfig struct {
	ap         *issecv1beta1.AuthorizationPolicy
	vs         *isnetv1beta1.VirtualService
	gateway    types.NamespacedName
	gwSelector map[string]string
	baseURL    string
}

func (ic *IstioConfig) GetVirtualService() *isnetv1beta1.VirtualService {
	return ic.vs
}

func (ic *IstioConfig) GetAuthorizationPolicy() *issecv1beta1.AuthorizationPolicy {
	return ic.ap
}

func (ic *IstioConfig) ConfigureVS(svc, es types.NamespacedName, endpoints map[string]common.NamedPath) {
	host := ic.baseURL
	pathPrefix := fmt.Sprintf("/%s/%s", es.Namespace, es.Name)
	svcHost := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)

	vs := isnetv1beta1.VirtualService{
		Spec: netv1beta1.VirtualService{
			Hosts:    []string{host},
			Gateways: []string{ic.gateway.String()},
		},
	}
	vs.Name = fmt.Sprintf("%s-%s", es.Namespace, es.Name)
	vs.Namespace = ic.gateway.Namespace

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
		}

		routes[index] = &route
		index++
	}

	if len(routes) == 0 {
		fmt.Println("no routes")
		return
	}

	vs.Spec.Http = routes
	ic.vs = &vs
}

func (ic *IstioConfig) ConfigureAP(nsn types.NamespacedName, inCIDRs []string, endpoints map[string]common.NamedPath) {

	cirds := make([]string, len(inCIDRs))
	copy(cirds, inCIDRs)

	pathPrefix := fmt.Sprintf("/%s/%s", nsn.Namespace, nsn.Name)
	matcher := maps.Clone(ic.gwSelector)

	ap := issecv1beta1.AuthorizationPolicy{
		Spec: secv1beta1.AuthorizationPolicy{
			Selector: &isv1beta1.WorkloadSelector{
				MatchLabels: matcher,
			},
		},
	}
	ap.Name = fmt.Sprintf("%s-%s", nsn.Namespace, nsn.Name)
	ap.Namespace = ic.gateway.Namespace

	paths := make([]string, len(endpoints))
	index := 0
	for _, path := range endpoints {
		paths[index] = pathPrefix + path.Path + "/*"
		index++
	}
	if len(paths) == 0 {
		return
	}

	rule := &secv1beta1.Rule{
		From: []*secv1beta1.Rule_From{
			&secv1beta1.Rule_From{
				Source: &secv1beta1.Source{
					IpBlocks: cirds,
				},
			},
		},
		To: []*secv1beta1.Rule_To{
			&secv1beta1.Rule_To{
				Operation: &secv1beta1.Operation{
					Hosts: []string{
						ic.baseURL,
						fmt.Sprintf("%s:*", ic.baseURL),
					},
					Paths: paths,
				},
			},
		},
	}

	ap.Spec.Rules = append(ap.Spec.Rules, rule)

	ic.ap = &ap
}
