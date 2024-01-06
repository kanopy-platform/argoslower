package istio

import (
	"fmt"
	"testing"

	common "github.com/kanopy-platform/argoslower/pkg/ingress"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestConfigureVS(t *testing.T) {
	ic := &IstioConfig{
		baseURL: "gateway.example.com",
		gateway: types.NamespacedName{
			Name:      "gateway",
			Namespace: "routing",
		},
	}

	tests := []struct {
		name      string
		svc       types.NamespacedName
		es        types.NamespacedName
		endpoints map[string]common.NamedPath
		isNil     bool
	}{
		{
			name:  "empty",
			isNil: true,
		},
		{
			name: "basic",
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
		ic.ConfigureVS(test.svc, test.es, test.endpoints)
		vs := ic.GetVirtualService()
		if test.isNil {
			assert.Nil(t, vs)
			continue
		}

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
	ic := &IstioConfig{
		baseURL: "gateway.example.com",
		gateway: types.NamespacedName{
			Name:      "gateway",
			Namespace: "routing",
		},
	}

	tests := []struct {
		name      string
		es        types.NamespacedName
		cidrs     []string
		endpoints map[string]common.NamedPath
		isNil     bool
	}{
		{
			name:  "empty",
			isNil: true,
		},
		{
			name: "basic",
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
		ic.ConfigureAP(test.es, test.cidrs, test.endpoints)
		ap := ic.GetAuthorizationPolicy()
		if test.isNil {
			assert.Nil(t, ap, test.name)
			continue
		}

		assert.Equal(t, test.cidrs, ap.Spec.Rules[0].From[0].Source.IpBlocks, test.name)
		assert.Equal(t, len(test.endpoints), len(ap.Spec.Rules[0].To[0].Operation.Paths), test.name)
		for _, endpoint := range test.endpoints {
			path := fmt.Sprintf("/%s/%s%s/*", test.es.Namespace, test.es.Name, endpoint.Path)
			assert.Contains(t, ap.Spec.Rules[0].To[0].Operation.Paths, path, test.name)
		}
	}
}
