package iplister

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	defaultTimeout = time.Minute
)

type IPLister struct {
	reader  Reader
	decoder Decoder
	timeout time.Duration
}

func New(reader Reader, decoder Decoder, opts ...iplisterOption) *IPLister {
	i := &IPLister{
		reader:  reader,
		decoder: decoder,
		timeout: defaultTimeout,
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

func (i *IPLister) GetIPs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), i.timeout)
	defer cancel()

	reader, err := i.reader.Data(ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	ipList, err := i.decoder.Decode(reader)
	if err != nil {
		return nil, err
	}

	if err := ValidateCIDRs(ipList); err != nil {
		return nil, err
	}

	return ipList, nil
}

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
