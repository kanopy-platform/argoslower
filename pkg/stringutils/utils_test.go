package stringutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringToMap(t *testing.T) {

	tests := map[string]struct {
		in       string
		delim    string
		split    string
		expected map[string]string
	}{
		"empty": {expected: map[string]string{}},
		"good": {
			in:    "a=b,c=d,e=f",
			delim: ",",
			split: "=",
			expected: map[string]string{
				"a": "b",
				"c": "d",
				"e": "f",
			},
		},
		"omittruncated": {
			in:    "a:b|truncated:",
			delim: "|",
			split: ":",
			expected: map[string]string{
				"a": "b",
			},
		},
		"omitmissing": {
			in:    "a:b|missing",
			delim: "|",
			split: ":",
			expected: map[string]string{
				"a": "b",
			},
		},
		"omitempty": {
			in:    "a:b|",
			delim: "|",
			split: ":",
			expected: map[string]string{
				"a": "b",
			},
		},
	}

	for name, test := range tests {

		out := StringToMap(test.in, test.delim, test.split)

		assert.Equal(t, len(test.expected), len(out))

		for k, ov := range out {
			ev, ok := test.expected[k]
			assert.True(t, ok, name)
			assert.Equal(t, ev, ov)
		}

	}
}
