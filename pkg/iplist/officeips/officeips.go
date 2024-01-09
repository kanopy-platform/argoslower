package officeips

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kanopy-platform/argoslower/pkg/iplist"
)

type OfficeIPs struct {
	url      string
	timeout  time.Duration
	username string
	password string
}

// Types for marshalling in response JSON
type (
	officeIP struct {
		CIDR string `json:"cidr"`
	}

	officeIPsResponse struct {
		OfficeIPs []officeIP `json:"office_ips"`
	}
)

func New(officeIPsURL, user, pass string, opts ...officeIPsOption) *OfficeIPs {
	o := &OfficeIPs{
		url:      officeIPsURL,
		timeout:  iplist.DefaultTimeout,
		username: user,
		password: pass,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *OfficeIPs) GetIPs() ([]string, error) {
	client := &http.Client{
		Timeout: o.timeout,
	}

	req, err := http.NewRequest("GET", o.url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(o.username, o.password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data officeIPsResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	ips := data.extractCIDR()

	if err := iplist.ValidateCIDRs(ips); err != nil {
		return nil, err
	}

	return ips, nil
}

func (o *officeIPsResponse) extractCIDR() []string {
	res := []string{}

	for _, officeIP := range o.OfficeIPs {
		res = append(res, officeIP.CIDR)
	}

	return res
}
