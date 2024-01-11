package file

import (
	"context"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testFile    = "iplists.yaml"
	testContent = `
iplists:
  jira:
    - "1.2.3.4/32"
    - "5.6.7.8/28"
  other:
    - "101.102.103.104/32"
    - "101.102.103.104/28"
`
)

func TestNew(t *testing.T) {
	t.Parallel()

	f := New(testFile)
	assert.Equal(t, testFile, f.filename)
}

func TestData(t *testing.T) {
	t.Parallel()

	tmpdir := t.TempDir()
	filepath := path.Join(tmpdir, testFile)
	tmpFile, err := os.Create(filepath)
	assert.NoError(t, err)

	_, err = tmpFile.Write([]byte(testContent))
	assert.NoError(t, err)

	f := New(filepath)
	readCloser, err := f.Data(context.Background())
	assert.NoError(t, err)
	defer readCloser.Close()

	result, err := io.ReadAll(readCloser)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(result[:]))
}
