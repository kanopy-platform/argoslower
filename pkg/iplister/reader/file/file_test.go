package file

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testFilename = "../../test_data/iplist.yaml"
)

func TestNew(t *testing.T) {
	t.Parallel()

	f := New(testFilename)
	assert.Equal(t, testFilename, f.filename)
}

func TestData(t *testing.T) {
	t.Parallel()

	f := New(testFilename)
	readCloser, err := f.Data(context.Background())
	assert.NoError(t, err)
	defer readCloser.Close()

	result, err := io.ReadAll(readCloser)
	assert.NoError(t, err)

	s := string(result[:])
	assert.Contains(t, s, "iplists")
}
