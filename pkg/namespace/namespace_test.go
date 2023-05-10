package namespace

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
)

type MockNamespaceLister struct {
	namespaces map[string]*corev1.Namespace
	err        error
}

func (m *MockNamespaceLister) List(selector labels.Selector) (ret []*corev1.Namespace, err error) {
	namespaces := []*corev1.Namespace{}

	for _, n := range m.namespaces {
		namespaces = append(namespaces, n)
	}

	return namespaces, m.err
}

func (m *MockNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if namespace, ok := m.namespaces[name]; ok {
		return namespace, m.err
	}

	return nil, m.err
}

func TestRateLimit(t *testing.T) {
	t.Parallel()

	rateLimitAnnotation := "kanopy-platform/events-rate-limit"

	userNamespaceWithAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					rateLimitAnnotation: "10",
				},
			},
		},
	}

	userNamespaceWithoutAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	userNamespaceWithInvalidAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					rateLimitAnnotation: "abc123",
				},
			},
		},
	}

	tests := []struct {
		testMsg       string
		namespaceName string
		mockLister    corev1Listers.NamespaceLister
		wantRateLimit *int
		wantError     bool
	}{
		{
			testMsg:       "successfully extract rate limit from namespace annotation",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: userNamespaceWithAnnotation,
				err:        nil,
			},
			wantRateLimit: pointer.Int(10),
			wantError:     false,
		},
		{
			testMsg:       "namespace annotation does not contain rate limit annotation",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: userNamespaceWithoutAnnotation,
				err:        nil,
			},
			wantRateLimit: nil,
			wantError:     false,
		},
		{
			testMsg:       "cannot find namespace",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{},
				err:        fmt.Errorf("namespace not found"),
			},
			wantRateLimit: nil,
			wantError:     true,
		},
		{
			testMsg:       "namespace annotation value is invalid",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: userNamespaceWithInvalidAnnotation,
				err:        nil,
			},
			wantRateLimit: nil,
			wantError:     true,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)
		n := NewNamespaceInfo(test.mockLister, rateLimitAnnotation)

		result, err := n.RateLimit(test.namespaceName)
		assert.Equal(t, test.wantRateLimit, result)
		assert.Equal(t, test.wantError, err != nil)
	}
}
