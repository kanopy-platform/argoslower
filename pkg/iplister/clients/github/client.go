package github

import (
	"github.com/kanopy-platform/argoslower/pkg/iplister"
	"github.com/kanopy-platform/argoslower/pkg/iplister/decoder/github"
	"github.com/kanopy-platform/argoslower/pkg/iplister/reader/http"
)

const GithubURL string = "https://api.github.com/meta"

func New() *iplister.IPLister {
	return iplister.New(http.New(GithubURL), github.New())
}
