package main

import (
	"fmt"
	"os"

	"github.com/kanopy-platform/argoslower/pkg/iplister"
	"github.com/kanopy-platform/argoslower/pkg/iplister/decoder/github"
	"github.com/kanopy-platform/argoslower/pkg/iplister/decoder/officeips"
	"github.com/kanopy-platform/argoslower/pkg/iplister/reader/http"
)

func main() {
	githubReader := http.New("https://api.github.com/meta")
	githubDecoder := github.New()

	githubIpLister := iplister.New(githubReader, githubDecoder)
	ips, err := githubIpLister.GetIPs()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Github IPs: %v\n", ips)

	officeipReader := http.New(os.Getenv("url"), http.WithBasicAuth(os.Getenv("user"), os.Getenv("pass")))
	officeipDecoder := officeips.New()
	officeipLister := iplister.New(officeipReader, officeipDecoder)
	ips, err = officeipLister.GetIPs()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Office IPs: %v\n", ips)
}
