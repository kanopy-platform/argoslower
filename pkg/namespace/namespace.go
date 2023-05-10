package namespace

import (
	"fmt"
	"strconv"

	corev1Listers "k8s.io/client-go/listers/core/v1"
)

type NamespaceInfo struct {
	lister              corev1Listers.NamespaceLister
	rateLimitAnnotation string
}

func NewNamespaceInfo(lister corev1Listers.NamespaceLister, rateLimitAnnotation string) *NamespaceInfo {
	return &NamespaceInfo{
		lister:              lister,
		rateLimitAnnotation: rateLimitAnnotation,
	}
}

// Retrieves the namespace and extracts the rate-limit annotation value if exists, nil otherwise.
func (n *NamespaceInfo) RateLimit(namespace string) (*int, error) {
	if namespace == "" {
		return nil, fmt.Errorf("invalid namespace; %q", namespace)
	}

	ns, err := n.lister.Get(namespace)
	if err != nil {
		return nil, err
	}

	if val, ok := ns.Annotations[n.rateLimitAnnotation]; ok {
		rateLimit, err := strconv.Atoi(val)
		if err != nil {
			return nil, err
		}

		return &rateLimit, nil
	}

	return nil, nil
}
