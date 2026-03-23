package core

// RouterStatus holds the current state of the home router/modem.
type RouterStatus struct {
	DSLDownMbit  float64
	DSLUpMbit    float64
	DSLOnline    bool
	LTEActive    bool
	LTESignalDBm float64
	LTEBand      string
	Mode         string // "DSL", "LTE", "Hybrid"
	Online       bool
}

// RouterFetcher is the common interface for all router backends.
type RouterFetcher interface {
	Fetch() (*RouterStatus, error)
}

// NewRouterFetcher returns the appropriate RouterFetcher for the given type.
// routerType must be "speedport" or "fritzbox".
// Falls back to FritzBox for unknown types.
func NewRouterFetcher(routerType, url, password string) RouterFetcher {
	switch routerType {
	case "speedport":
		if url == "" {
			url = "http://192.168.2.1"
		}
		return &SpeedportFetcher{BaseURL: url, Password: password}
	default:
		if url == "" {
			url = "http://fritz.box:49000"
		}
		return &FritzBoxFetcher{BaseURL: url}
	}
}
