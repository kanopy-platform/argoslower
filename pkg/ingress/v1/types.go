package v1

import (
	"context"

	ingresscommon "github.com/kanopy-platform/argoslower/pkg/ingress"
	"k8s.io/apimachinery/pkg/types"
)

// EventSourceIngressConfig provides the information needed for rendering
// ingress resources mapped to the service of an argo event source.
// it is ingress provider agnostic.
type EventSourceIngressConfig struct {
	Ipg            IPGetter
	Es             types.NamespacedName
	Endpoints      map[string]ingresscommon.NamedPath
	AdminNamespace string
	BaseURL        string
	Gateway        types.NamespacedName
	Service        types.NamespacedName
}

// IPGetter defines an interface for ip address providers to get source CIDRs
type IPGetter interface {
	GetIPs() ([]string, error)
}

type IngressConfigurator interface {
	Configure(ctx context.Context, config *EventSourceIngressConfig) ([]types.NamespacedName, error)
	Remove(ctx context.Context, config *EventSourceIngressConfig) error
}
