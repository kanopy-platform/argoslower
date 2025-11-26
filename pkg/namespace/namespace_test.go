package namespace

import (
	"fmt"
	"testing"

	sensor "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1Listers "k8s.io/client-go/listers/core/v1"
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

	rateLimitUnitAnnotation := "rate-limit-unit"
	requestsPerUnitAnnotation := "requests-per-unit"

	correctAnnotations := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					rateLimitUnitAnnotation:   "Minute",
					requestsPerUnitAnnotation: "10",
				},
			},
		},
	}

	missingAnnotations := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	missingUnitAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					requestsPerUnitAnnotation: "50",
				},
			},
		},
	}

	invalidUnitAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					rateLimitUnitAnnotation: "Months",
				},
			},
		},
	}

	invalidRequestsAnnotation := map[string]*corev1.Namespace{
		"user": &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					requestsPerUnitAnnotation: "abc123",
				},
			},
		},
	}

	tests := []struct {
		testMsg       string
		namespaceName string
		mockLister    corev1Listers.NamespaceLister
		wantResult    *sensor.RateLimit
		wantError     bool
	}{
		{
			testMsg:       "successfully extract rate limit unit and requests from annotations",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: correctAnnotations,
				err:        nil,
			},
			wantResult: &sensor.RateLimit{Unit: sensor.Minute, RequestsPerUnit: 10},
			wantError:  false,
		},
		{
			testMsg:       "rate limit unit and requests annotations missing",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: missingAnnotations,
				err:        nil,
			},
			wantResult: nil,
			wantError:  false,
		},
		{
			testMsg:       "rate limit unit and annotation missing, default seconds",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: missingUnitAnnotation,
				err:        nil,
			},
			wantResult: &sensor.RateLimit{Unit: sensor.Second, RequestsPerUnit: 50},
			wantError:  false,
		},
		{
			testMsg:       "cannot find namespace",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{},
				err:        fmt.Errorf("namespace not found"),
			},
			wantResult: nil,
			wantError:  true,
		},
		{
			testMsg:       "unit annotation value is invalid",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: invalidUnitAnnotation,
				err:        nil,
			},
			wantResult: nil,
			wantError:  true,
		},
		{
			testMsg:       "requests annotation value is invalid",
			namespaceName: "user",
			mockLister: &MockNamespaceLister{
				namespaces: invalidRequestsAnnotation,
				err:        nil,
			},
			wantResult: nil,
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Log(test.testMsg)
		n := NewNamespaceInfo(test.mockLister, rateLimitUnitAnnotation, requestsPerUnitAnnotation)

		result, err := n.RateLimit(test.namespaceName)
		assert.Equal(t, test.wantResult, result)
		assert.Equal(t, test.wantError, err != nil)
	}
}
