package file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testFilename = "../../test_data/iplist.yaml"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	testFile, err := os.Open(testFilename)
	assert.NoError(t, err)

	f := New("jira")
	res, err := f.Decode(testFile)
	assert.NoError(t, err)
	assert.Equal(t, []string{"1.2.3.4/32", "5.6.7.8/28"}, res)
}
