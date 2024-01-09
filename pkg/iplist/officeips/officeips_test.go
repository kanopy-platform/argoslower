package officeips

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithURL(t *testing.T) {
	t.Parallel()

	testUrl := "test.example.com"

	g := New(WithURL(testUrl))
	assert.Equal(t, testUrl, g.url)
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	testTimeout := time.Hour * 123

	g := New(WithTimeout(testTimeout))
	assert.Equal(t, testTimeout, g.timeout)
}

func TestGetIPs(t *testing.T) {
	t.Parallel()

	g := New()
	res, err := g.GetIPs()

	assert.NoError(t, err)
	assert.NotEmpty(t, res)
}
