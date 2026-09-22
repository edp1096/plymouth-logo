package main

import (
	"strings"
	"testing"
)

func TestAppWindowArguments(t *testing.T) {
	url := "http://127.0.0.1:8090/#test-token"
	args := windowArgs(url, "/tmp/profile with spaces")
	if args[0] != "--app="+url || args[1] != "--user-data-dir=/tmp/profile with spaces" {
		t.Fatal(args)
	}
	for _, arg := range args {
		if arg == url || strings.HasPrefix(arg, "--no-sandbox") {
			t.Fatalf("unexpected argument: %s", arg)
		}
	}
}
