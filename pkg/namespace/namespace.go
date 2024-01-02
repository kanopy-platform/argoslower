package namespace

import (
	"fmt"
	"strconv"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	corev1Listers "k8s.io/client-go/listers/core/v1"
)

type NamespaceInfo struct {
	lister                    corev1Listers.NamespaceLister
	rateLimitUnitAnnotation   string
	requestsPerUnitAnnotation string
}

func NewNamespaceInfo(lister corev1Listers.NamespaceLister, rateLimitUnitAnnotation, requestsPerUnitAnnotation string) *NamespaceInfo {
	return &NamespaceInfo{
		lister:                    lister,
		rateLimitUnitAnnotation:   rateLimitUnitAnnotation,
		requestsPerUnitAnnotation: requestsPerUnitAnnotation,
	}
}

// Retrieves the namespace RateLimit values if exists, nil otherwise.
func (n *NamespaceInfo) RateLimit(namespace string) (*sensor.RateLimit, error) {
	if namespace == "" {
		return nil, fmt.Errorf("invalid namespace; %q", namespace)
	}

	ns, err := n.lister.Get(namespace)
	if err != nil {
		return nil, err
	}

	result := sensor.RateLimit{}

	if val, ok := ns.Annotations[n.rateLimitUnitAnnotation]; ok {
		unit := sensor.RateLimiteUnit(val)
		switch unit {
		case sensor.Second, sensor.Minute, sensor.Hour:
			result.Unit = unit
		default:
			return nil, fmt.Errorf("invalid %s: %s", n.rateLimitUnitAnnotation, val)
		}
	} else {
		result.Unit = sensor.Second
	}

	if str, ok := ns.Annotations[n.requestsPerUnitAnnotation]; ok {
		val, err := strconv.Atoi(str)
		if err != nil {
			return nil, err
		}
		result.RequestsPerUnit = int32(val)
	} else {
		// return nil indicating requestsPerUnit not set
		return nil, nil
	}

	return &result, nil
}

func (n *NamespaceInfo) OnMesh(namespace string) (bool, error) {
	if namespace == "" {
		return false, nil
	}

	ns, err := n.lister.Get(namespace)
	if err != nil {
		return false, err
	}

	if val, ok := ns.Labels["istio.io/rev"]; !ok || val == "" {
		return false, nil
	}
	return true, nil
}
