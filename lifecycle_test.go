package main

import (
	"testing"
	"time"
)

func TestWindowClose(t *testing.T) {
	done := make(chan struct{}, 1)
	w := &windowWatch{delay: 10 * time.Millisecond, quit: func() { done <- struct{}{} }}
	w.connect()
	w.disconnect()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("window close did not stop app")
	}
}
func TestWindowReload(t *testing.T) {
	done := make(chan struct{}, 1)
	w := &windowWatch{delay: 20 * time.Millisecond, quit: func() { done <- struct{}{} }}
	w.connect()
	w.disconnect()
	w.connect()
	select {
	case <-done:
		t.Fatal("reload stopped app")
	case <-time.After(50 * time.Millisecond):
	}
	w.disconnect()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("final disconnect ignored")
	}
}
