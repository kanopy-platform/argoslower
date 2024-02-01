package stringutils

import "strings"

// StringToMap splits a string of delim delimited key value pairs into a lookup map
// based on the split value. i.e. "example=one,string=two" yields a map with keys
// example: one, string: two
func StringToMap(in, delim, split string) map[string]string {
	out := map[string]string{}
	for _, v := range strings.Split(in, delim) {
		mark := strings.Index(v, split)
		if mark < 0 || mark+1 >= len(v) {
			continue
		}
		out[v[:mark]] = v[mark+1:]
	}

	return out
}
