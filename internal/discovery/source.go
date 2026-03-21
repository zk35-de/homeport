package discovery

// Source is implemented by each discovery backend.
type Source interface {
	Type() string
	Fetch() ([]DiscoveredService, error)
}

// DiscoveredService is a service found by a discovery source.
type DiscoveredService struct {
	ExternalID  string // stable ID from the source (NPM proxy-host ID, Docker container ID)
	Name        string
	URL         string
	Description string
	Icon        string
}
