package iplister

import (
	"context"
	"io"
)

type (
	// Fetches iplist data from a webpage or file
	Reader interface {
		Data(context.Context) (io.ReadCloser, error)
	}

	// Parses the input containing iplist data and returns a list of CIDRs
	Decoder interface {
		Decode(io.ReadCloser) ([]string, error)
	}
)
