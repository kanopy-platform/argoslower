package officeips

import (
	"encoding/json"
	"io"
)

type OfficeIPs struct{}

// Structs for marshalling in data from officeips endpoint
type (
	officeIP struct {
		CIDR string `json:"cidr"`
	}

	officeIPsResponse struct {
		OfficeIPs []officeIP `json:"office_ips"`
	}
)

func New() *OfficeIPs {
	return &OfficeIPs{}
}

func (o *OfficeIPs) Decode(data io.ReadCloser) ([]string, error) {
	var resp officeIPsResponse
	err := json.NewDecoder(data).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp.extractCIDRs(), nil
}

func (o *officeIPsResponse) extractCIDRs() []string {
	res := []string{}

	for _, officeIP := range o.OfficeIPs {
		res = append(res, officeIP.CIDR)
	}

	return res
}
