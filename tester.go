package main

import (
	"fmt"

	"github.com/kanopy-platform/argoslower/pkg/iplister"
	filedeocder "github.com/kanopy-platform/argoslower/pkg/iplister/decoder/file"
	"github.com/kanopy-platform/argoslower/pkg/iplister/reader/file"
)

func main() {
	// githubIpLister := iplister.New(
	// 	http.New("https://api.github.com/meta"),
	// 	github.New(),
	// )
	// ips, err := githubIpLister.GetIPs()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Github IPs: %v\n", ips)

	// officeipLister := iplister.New(
	// 	http.New(os.Getenv("url"), http.WithBasicAuth(os.Getenv("user"), os.Getenv("pass"))),
	// 	officeips.New(),
	// )
	// ips, err = officeipLister.GetIPs()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Office IPs: %v\n", ips)

	file := iplister.New(
		file.New("iplists.yaml"),
		filedeocder.New("jira"),
	)
	ips, err := file.GetIPs()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Jira IPs: %v\n", ips)
}
