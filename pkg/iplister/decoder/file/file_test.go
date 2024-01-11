package file

import (
	"bytes"
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

	testContent := `
iplists:
  jira:
    - "1.2.3.4/32"
    - "5.6.7.8/28"
  other:
    - "101.102.103.104/32"
    - "101.102.103.104/28"
`

	f := New("jira")
	res, err := f.Decode(newMockReadCloser([]byte(testContent)))
	assert.NoError(t, err)
	assert.Equal(t, []string{"1.2.3.4/32", "5.6.7.8/28"}, res)
}
