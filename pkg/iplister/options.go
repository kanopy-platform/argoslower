package iplister

import "time"

type iplisterOption func(*IPLister)

func WithTimeout(timeout time.Duration) iplisterOption {
	return func(i *IPLister) {
		i.timeout = timeout
	}
}
