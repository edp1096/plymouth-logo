package main

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguagePreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	request := func(a *app, method, body, token, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost/api/preferences", strings.NewReader(body))
		r.Header.Set("X-App-Token", token)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	a := &app{host: "localhost", token: "secret", preferencesPath: path}
	if w := request(a, "POST", `{"language":"en"}`, "", "http://localhost"); w.Code != 403 {
		t.Fatal("unauthenticated write", w.Code)
	}
	if w := request(a, "POST", `{"language":"en"}`, "secret", "http://other"); w.Code != 403 {
		t.Fatal("cross-origin write", w.Code)
	}
	if w := request(a, "POST", `{"language":"xx"}`, "secret", ""); w.Code != 400 {
		t.Fatal("invalid language", w.Code)
	}
	if w := request(a, "POST", `{"language":"en"}`, "secret", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	restarted := &app{host: "localhost", token: "new", preferencesPath: path}
	if w := request(restarted, "GET", "", "new", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"en"`) {
		t.Fatal("language did not persist", w.Code, w.Body.String())
	}
}
