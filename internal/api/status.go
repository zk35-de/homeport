package api

import (
	"encoding/json"
	"fmt"
	"log"
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

var StatusBroker = &Broker{
	Clients:  make(map[chan string]bool),
	Add:      make(chan chan string),
	Remove:   make(chan chan string),
	Messages: make(chan string),
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

func StartStatusChecker() {
	go StatusBroker.Start()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Initial check
		checkAllServices()

		for range ticker.C {
			checkAllServices()
		}
	}()
}

func checkAllServices() {
	services, err := db.GetAllServicesWithStatusCheck()
	if err != nil {
		log.Printf("Error fetching services for status check: %v", err)
		return
	}

	for _, s := range services {
		go checkService(s)
	}
}

func checkService(s db.Service) {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Head(s.StatusCheck)
	alive := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400
	if resp != nil {
		resp.Body.Close()
	}

	if err := db.UpdateServiceStatus(s.ID, alive); err != nil {
		log.Printf("Error updating status for service %d: %v", s.ID, err)
	}

	update := StatusUpdate{
		ID:    s.ID,
		Alive: alive,
	}
	msg, _ := json.Marshal(update)
	StatusBroker.Messages <- string(msg)

	DefaultHub.Broadcast(Message{
		Type:    ServiceStatusMsg,
		Payload: update,
	})
}

func HandleStatusStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)
	StatusBroker.Add <- messageChan

	defer func() {
		StatusBroker.Remove <- messageChan
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
