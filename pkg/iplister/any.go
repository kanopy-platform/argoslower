package iplister

type AnyGetter struct {
}

func (a AnyGetter) GetIPs() ([]string, error) {
	return []string{"0.0.0.0/0"}, nil
}
