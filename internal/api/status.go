package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"git.zk35.de/secalpha/homeport/internal/db"
)

type StatusUpdate struct {
	ID    int  `json:"id"`
	Alive bool `json:"alive"`
}

type Broker struct {
	Clients  map[chan string]bool
	Add      chan chan string
	Remove   chan chan string
	Messages chan string
}

func (b *Broker) Start() {
	for {
		select {
		case s := <-b.Add:
			b.Clients[s] = true
		case s := <-b.Remove:
			delete(b.Clients, s)
			close(s)
		case msg := <-b.Messages:
			for s := range b.Clients {
				select {
				case s <- msg:
				default:
					// Skip if channel is blocked to prevent hanging
				}
			}
		}
	}
}

func (s *Server) StartStatusChecker() {
	go s.Broker.Start()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Initial check
		s.checkAllServices()

		for range ticker.C {
			s.checkAllServices()
		}
	}()
}

type serviceResult struct {
	id    int
	alive bool
}

func (s *Server) checkAllServices() {
	services, err := db.GetAllServicesWithStatusCheck()
	if err != nil {
		slog.Error("fetching services for status check", "err", err)
		return
	}

	results := make(chan serviceResult, len(services))
	for _, svc := range services {
		go func(svc db.Service) {
			results <- serviceResult{id: svc.ID, alive: pingService(svc.StatusCheck)}
		}(svc)
	}

	for range services {
		r := <-results
		if err := db.UpdateServiceStatus(r.id, r.alive); err != nil {
			slog.Error("updating service status", "id", r.id, "err", err)
		}
		update := StatusUpdate{ID: r.id, Alive: r.alive}
		msg, _ := json.Marshal(update)
		s.Broker.Messages <- string(msg)
		s.Hub.Broadcast(Message{Type: ServiceStatusMsg, Payload: update})
	}
}

func pingService(url string) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(url)
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}

func (s *Server) HandleStatusStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)
	s.Broker.Add <- messageChan

	defer func() {
		s.Broker.Remove <- messageChan
	}()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		}
	}
}
