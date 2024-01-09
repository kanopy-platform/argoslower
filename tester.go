package main

import (
	"fmt"

	"github.com/kanopy-platform/argoslower/pkg/iplist/github"
	"github.com/kanopy-platform/argoslower/pkg/iplist/officeips"
)

func main() {
	github := github.New()
	ips, err := github.GetIPs()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Github IPs: %v\n", ips)

	officips := officeips.New()
	ips, err = officips.GetIPs()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Office IPs: %v\n", ips)
}
