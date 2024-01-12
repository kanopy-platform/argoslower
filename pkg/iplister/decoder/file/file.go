package file

import (
	"io"

	"gopkg.in/yaml.v3"
)

type File struct {
	sources []string
}

// Structs for marshalling in data from an ipLists file
type (
	ipLists struct {
		IPLists map[string][]string `json:"ipLists"`
	}
)

func New(sources ...string) *File {
	return &File{
		sources: sources,
	}
}

func (f *File) Decode(data io.ReadCloser) ([]string, error) {
	var resp ipLists
	err := yaml.NewDecoder(data).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp.extractCIDRs(f.sources), nil
}

func (i ipLists) extractCIDRs(sources []string) []string {
	res := []string{}

	for _, s := range sources {
		val, ok := i.IPLists[s]
		if ok {
			res = append(res, val...)
		}
	}

	return res
}
