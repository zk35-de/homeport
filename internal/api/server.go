package api

import (
	"embed"
	"html/template"
	"log"

	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/i18n"
)

// Server holds all shared dependencies for the API handlers.
type Server struct {
	Config        *config.Config
	Hub           *UpdateHub
	Broker        *Broker
	IndexTmpl     *template.Template
	ManageTmpl    *template.Template
	AnalyticsTmpl *template.Template
	LoginTmpl     *template.Template
}

// New creates a Server with the given config and initialised Hub/Broker.
func New(cfg *config.Config) *Server {
	return &Server{
		Config: cfg,
		Hub:    NewUpdateHub(),
		Broker: &Broker{
			Clients:  make(map[chan string]bool),
			Add:      make(chan chan string),
			Remove:   make(chan chan string),
			Messages: make(chan string),
		},
	}
}

// InitTemplates loads all HTML templates from the embedded FS into the Server.
func (s *Server) InitTemplates(fs embed.FS) {
	i18n.Load(fs)
	var err error

	s.IndexTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
		"templates/base.html",
		"templates/index.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing index templates: %v", err)
	}

	s.ManageTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
		"templates/base.html",
		"templates/manage.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing manage templates: %v", err)
	}

	s.AnalyticsTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
		"templates/base.html",
		"templates/analytics.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing analytics templates: %v", err)
	}

	s.LoginTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs, "templates/login.html")
	if err != nil {
		log.Fatalf("Error parsing login template: %v", err)
	}
}
