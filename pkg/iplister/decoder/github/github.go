package github

import (
	"encoding/json"
	"io"
)

// Structs for marshalling in data from https://api.github.com/meta
type (
	githubMetaAPI struct {
		Hooks []string `json:"hooks"`
	}
)

type Github struct{}

func New() *Github {
	return &Github{}
}

func (g *Github) Decode(data io.ReadCloser) ([]string, error) {
	var resp githubMetaAPI

	err := json.NewDecoder(data).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp.Hooks, nil
}
