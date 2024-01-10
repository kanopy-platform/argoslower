package iplist

import (
	"errors"
	"net"
	"time"
)

const (
	DefaultTimeout = time.Minute
)

func ValidateCIDRs(list []string) error {
	errs := []error{}

	for _, cidr := range list {
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
