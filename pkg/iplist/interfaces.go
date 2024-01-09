package iplist

// TODO delete this. This is just a reminder of the signature

// IPGetter defines an interface for ip address providers to get source CIDRs
type IPGetter interface {
	GetIPs() ([]string, error)
}
