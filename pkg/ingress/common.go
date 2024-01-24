package ingress

const (
	EventSourceNameString      string = "eventsource-name"
	EventSourceNamespaceString string = "eventsource-namespace"
)

type NamedPath struct {
	Name string
	Path string
}
