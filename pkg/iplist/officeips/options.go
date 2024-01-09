package officeips

import "time"

type officeIPsOption func(*OfficeIPs)

func WithURL(url string) officeIPsOption {
	return func(o *OfficeIPs) {
		o.url = url
	}
}

func WithTimeout(timeout time.Duration) officeIPsOption {
	return func(o *OfficeIPs) {
		o.timeout = timeout
	}
}
