package officeips

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithURL(t *testing.T) {
	t.Parallel()

	testUrl := "test.example.com"

	g := New("example.com", "user", "pass", WithURL(testUrl))
	assert.Equal(t, testUrl, g.url)
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	testTimeout := time.Hour * 123

	g := New("example.com", "user", "pass", WithTimeout(testTimeout))
	assert.Equal(t, testTimeout, g.timeout)
}

func TestGetIPs(t *testing.T) {
	t.Parallel()

	fakeResponse := officeIPsResponse{
		OfficeIPs: []officeIP{
			{
				CIDR: "1.2.3.4/32",
			},
			{
				CIDR: "123.4.5.6/11",
			},
		},
	}

	fakeData, err := json.Marshal(&fakeResponse)
	assert.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, err := res.Write(fakeData)
		assert.NoError(t, err)
	}))

	g := New("example.com", "user", "pass", WithURL(testServer.URL))
	res, err := g.GetIPs()

	assert.NoError(t, err)
	assert.Len(t, res, len(fakeResponse.OfficeIPs))
}
