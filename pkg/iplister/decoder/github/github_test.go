package github

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newMockReadCloser(b []byte) io.ReadCloser {
	reader := bytes.NewReader(b)
	return io.NopCloser(reader)
}

func TestDecode(t *testing.T) {
	t.Parallel()

	fakeResponse := githubMetaAPI{
		Hooks: []string{
			"1.2.3.4/32",
			"123.4.5.6/11",
		},
	}

	fakeData, err := json.Marshal(&fakeResponse)
	assert.NoError(t, err)

	g := New()
	res, err := g.Decode(newMockReadCloser(fakeData))

	assert.NoError(t, err)
	assert.Len(t, res, len(fakeResponse.Hooks))
}
