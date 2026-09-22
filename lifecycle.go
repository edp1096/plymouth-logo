package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// A live browser connection owns the app lifetime. Brief reloads get a grace period.
type windowWatch struct {
	mu         sync.Mutex
	active     int
	generation uint64
	delay      time.Duration
	quit       func()
}

func (v *windowWatch) connect() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.active++
	v.generation++
}
func (v *windowWatch) disconnect() {
	v.mu.Lock()
	v.active--
	v.generation++
	generation := v.generation
	v.mu.Unlock()
	time.AfterFunc(v.delay, func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		if v.active == 0 && v.generation == generation {
			v.quit()
		}
	})
}
func (v *windowWatch) serve(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unavailable", 500)
		return
	}
	v.connect()
	defer v.disconnect()
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, "data: connected\n\n")
	f.Flush()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": alive\n\n"); err != nil {
				return
			}
			f.Flush()
		}
	}
}
