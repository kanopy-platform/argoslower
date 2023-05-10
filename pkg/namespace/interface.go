package namespace

type RateLimitGetter interface {
	RateLimit(namespace string) (*int, error)
}
