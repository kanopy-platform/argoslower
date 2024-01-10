package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testUrl = "https://example.com"
)

func TestNew(t *testing.T) {
	t.Parallel()

	h := New(testUrl)
	assert.Equal(t, testUrl, h.url)
}

func TestWithBasicAuth(t *testing.T) {
	t.Parallel()

	username := "abc"
	password := "123"

	h := New(testUrl, WithBasicAuth(username, password))
	assert.Equal(t, username, h.username)
	assert.Equal(t, password, h.password)
}

func TestData(t *testing.T) {
	t.Parallel()

	fakeResponse := []byte("test string")

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, err := res.Write(fakeResponse)
		assert.NoError(t, err)
	}))

	h := New(testServer.URL)
	readCloser, err := h.Data(context.Background())
	assert.NoError(t, err)
	defer readCloser.Close()

	result, err := io.ReadAll(readCloser)
	assert.NoError(t, err)
	assert.Equal(t, fakeResponse, result)
}
