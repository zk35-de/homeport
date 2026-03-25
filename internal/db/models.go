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
	Alive       bool
	LastCheck   string
	VisibleTo   []string
}

type Widget struct {
	ID        int
	Type      string
	Name      string
	Config    string
	Profile   string
	SortOrder int
	PageID    int
	Events    []ICalEvent    `json:"-"`
	Todos         []TodoItem     `json:"-"`
	BookmarkLinks []BookmarkLink `json:"-"`
	NoteContent   string         `json:"-"`
	ClockMode        string `json:"-"`
	ClockTimezone    string `json:"-"`
	ClockShowSeconds bool   `json:"-"`
	ClockShowDate    bool   `json:"-"`
	ClockCountdown   string `json:"-"`
}

type BookmarkLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon"`
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

type TodoItem struct {
	ID        int    `json:"id"`
	WidgetID  int    `json:"widget_id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	DueDate   string `json:"due_date"`
	SortOrder int    `json:"sort_order"`
}

type ICalEvent struct {
	Title      string
	Start      string
	End        string
	IsToday    bool
	IsTomorrow bool
}

type WidgetCacheEntry struct {
	Events []ICalEvent `json:"Events,omitempty"`
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
