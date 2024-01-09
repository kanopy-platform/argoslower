package iplist

import (
	"net"
	"time"
)

const (
	DefaultTimeout = time.Minute
)

func ValidateCIDRs(list []string) error {
	for _, cidr := range list {
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
	}

	return nil
}
