package http

type httpOption func(*HTTP)

func WithBasicAuth(user, pass string) httpOption {
	return func(h *HTTP) {
		h.username = user
		h.password = pass
	}
}
