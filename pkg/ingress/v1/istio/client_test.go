package istio

import (
	"fmt"
	"testing"

	common "github.com/kanopy-platform/argoslower/pkg/ingress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestConfigureVS(t *testing.T) {
	ic := NewIstioConfig()

	tests := []struct {
		name      string
		baseURL   string
		gateway   types.NamespacedName
		svc       types.NamespacedName
		es        types.NamespacedName
		endpoints map[string]common.NamedPath
		err       bool
	}{
		{
			name:    "empty",
			baseURL: "gateway.example.com",
			gateway: types.NamespacedName{
				Name:      "gateway",
				Namespace: "routing",
			},
			err: true,
		},
		{
			name:    "basic",
			baseURL: "gateway.example.com",
			gateway: types.NamespacedName{
				Name:      "gateway",
				Namespace: "routing",
			},
			svc: types.NamespacedName{
				Name:      "upstreamservice",
				Namespace: "destination",
			},
			es: types.NamespacedName{
				Name:      "eventsource",
				Namespace: "destination",
			},
			endpoints: map[string]common.NamedPath{
				"12345": common.NamedPath{
					Name: "thingOne",
					Path: "/t1",
				},
				"54321": common.NamedPath{
					Name: "thingTwo",
					Path: "/t2",
				},
			},
		},
	}

	for _, test := range tests {
		err := ic.ConfigureVS(test.baseURL, test.gateway, test.svc, test.es, test.endpoints)
		if test.err {
			assert.Error(t, err, test.name)
			continue
		}
		assert.NoError(t, err, test.name)

		vs := ic.GetVirtualService()
		require.NotNil(t, vs, test.name)

		assert.Equal(t, test.es.Name, vs.Labels[common.EventSourceNameString], test)
		assert.Equal(t, test.es.Namespace, vs.Labels[common.EventSourceNamespaceString], test)

		for _, route := range vs.Spec.Http {
			// destination should be the fully qualified internal service name
			assert.Equal(t, fmt.Sprintf("%s.%s.svc.cluster.local", test.svc.Name, test.svc.Namespace), route.Route[0].Destination.Host, test.name)

			// detination port should 1. appear in the endpoint map 2. match the /namespace/eventsourcename/endpoint/ prefix match for the endpoint
			ep, ok := test.endpoints[fmt.Sprintf("%d", route.Route[0].Destination.Port.Number)]
			assert.True(t, ok, test.name)
			urlPrefix := route.Match[0].Uri.GetPrefix()
			assert.Equal(t, fmt.Sprintf("/%s/%s%s/", test.es.Namespace, test.es.Name, ep.Path), urlPrefix, test.name)
		}
	}
}

func TestConfigureAP(t *testing.T) {
	ic := NewIstioConfig()

	tests := []struct {
		name      string
		adminNS   string
		baseURL   string
		gws       map[string]string
		es        types.NamespacedName
		cidrs     []string
		endpoints map[string]common.NamedPath
		err       bool
	}{
		{
			name:    "empty",
			adminNS: "routing",
			baseURL: "gateway.example.com",
			gws: map[string]string{
				"istio": "example-ingressgateway",
			},
			err: true,
		},
		{
			name:    "basic",
			adminNS: "routing",
			baseURL: "gateway.example.com",
			gws: map[string]string{
				"istio": "example-ingressgateway",
			},
			es: types.NamespacedName{
				Name:      "eventsource",
				Namespace: "destination",
			},
			cidrs: []string{
				"1.2.3.4",
				"2.3.4.5/24",
			},
			endpoints: map[string]common.NamedPath{
				"12345": common.NamedPath{
					Name: "thing",
					Path: "/thingtarget",
				},
			},
		},
	}

	for _, test := range tests {
		err := ic.ConfigureAP(test.adminNS, test.baseURL, test.es, test.cidrs, test.endpoints, test.gws)
		if test.err {
			assert.Error(t, err, test.name)
			continue
		}

		assert.NoError(t, err, test.name)

		ap := ic.GetAuthorizationPolicy()
		require.NotNil(t, ap, test.name)

		assert.Equal(t, test.adminNS, ap.Namespace, test.name)
		assert.Equal(t, test.cidrs, ap.Spec.Rules[0].From[0].Source.NotIpBlocks, test.name)

		assert.Equal(t, test.es.Name, ap.Labels[common.EventSourceNameString], test)
		assert.Equal(t, test.es.Namespace, ap.Labels[common.EventSourceNamespaceString], test)

		assert.Equal(t, len(test.endpoints), len(ap.Spec.Rules[0].To[0].Operation.Paths), test.name)
		for _, endpoint := range test.endpoints {
			path := fmt.Sprintf("/%s/%s%s/*", test.es.Namespace, test.es.Name, endpoint.Path)
			assert.Contains(t, ap.Spec.Rules[0].To[0].Operation.Paths, path, test.name)
		}
	}
}
