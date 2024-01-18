package http

import (
	"context"
	"io"
	"net/http"
)

type HTTP struct {
	url      string
	username string
	password string
}

func New(url string, opts ...httpOption) *HTTP {
	h := &HTTP{
		url: url,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *HTTP) Data(ctx context.Context) (io.ReadCloser, error) {
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return nil, err
	}

	if h.username != "" || h.password != "" {
		req.SetBasicAuth(h.username, h.password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}
