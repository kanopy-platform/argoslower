package github

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kanopy-platform/argoslower/pkg/iplist"
)

const (
	githubMetaURL = "https://api.github.com/meta"
)

type Github struct {
	url     string
	timeout time.Duration
}

// Types for marshalling in response JSON
type (
	githubAPIResponse struct {
		Hooks []string `json:"hooks"`
	}
)

func New(opts ...githubOption) *Github {
	g := &Github{
		url:     githubMetaURL,
		timeout: iplist.DefaultTimeout,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

func (g *Github) GetIPs() ([]string, error) {
	client := &http.Client{
		Timeout: g.timeout,
	}

	resp, err := client.Get(g.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data githubAPIResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	ips := data.Hooks

	if err := iplist.ValidateCIDRs(ips); err != nil {
		return nil, err
	}

	return ips, nil
}
