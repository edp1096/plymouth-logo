package main

import (
	"encoding/json"
	"net/http"
	"os"
)

type preferences struct {
	Language string `json:"language"`
}

func (a *app) preferences(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var p preferences
	if r.Method == "GET" {
		data, err := os.ReadFile(a.preferencesPath)
		if err == nil {
			_ = json.Unmarshal(data, &p)
		}
		if p.Language != "ko" && p.Language != "en" {
			p.Language = ""
		}
		reply(w, 200, p)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Method not allowed")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+a.host {
		fail(w, 403, "Invalid origin")
		return
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&p); err != nil || (p.Language != "ko" && p.Language != "en") {
		fail(w, 400, "Language must be ko or en")
		return
	}
	data, _ := json.Marshal(p)
	if a.preferencesPath == "" {
		fail(w, 500, "Preferences unavailable")
		return
	}
	if err := atomicWrite(a.preferencesPath, data, 0600); err != nil {
		fail(w, 500, "Could not save language preference")
		return
	}
	reply(w, 200, p)
}
