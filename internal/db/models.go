package db

// Profile represents a homeport user profile.
type Profile struct {
	ID        int
	Slug      string
	Name      string
	IsDefault bool
	SortOrder int
}

type Page struct {
	ID        int
	Profile   string
	Name      string
	Icon      string
	SortOrder int
}

type Category struct {
	ID        int
	Name      string

	Color     string
	SortOrder int
	ColSpan   int
	SortMode  string
	PageID    int
	Services  []Service
}

type Service struct {
	ID          int
	CategoryID  int
	Name        string
	URL         string
	Icon        string
	Description string
	StatusCheck string
	SortOrder   int
	NoCheck     bool
	Alive       bool
	LastCheck   string
	VisibleTo   []string
}

type ClickStat struct {
	ServiceID   int
	ServiceName string
	ServiceURL  string
	ServiceIcon string
	ClickCount  int
	LastClicked string
	Profile     string
}

// ReorderItem is used for batch reorder operations.
type ReorderItem struct {
	ID        int `json:"id"`
	SortOrder int `json:"sort_order"`
}

// UserPreferences holds per-profile UI preferences.
type UserPreferences struct {
	Profile        string `json:"profile"`
	Theme          string `json:"theme"`
	AccentColor    string `json:"accent_color"`
	SearchEngine   string `json:"search_engine"`
	Language       string `json:"language"`

	CustomCSS      string `json:"custom_css"`

}

type DiscoveryItem struct {
	ID          int
	ContainerID string
	Suggested   SuggestedService
	SeenAt      string
}

type SuggestedService struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Profile     string `json:"profile"`
	StatusCheck string `json:"status_check"`
}
