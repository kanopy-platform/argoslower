package github

import "time"

type githubOption func(*Github)

func WithURL(url string) githubOption {
	return func(g *Github) {
		g.url = url
	}
}

func WithTimeout(timeout time.Duration) githubOption {
	return func(g *Github) {
		g.timeout = timeout
	}
}
