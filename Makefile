.DEFAULT_GOAL := build

GO ?= go
ARGS ?=

.PHONY: build run test race vet check fmt clean install uninstall help

build:
	mkdir -p dist
	$(GO) build -trimpath -o dist/plymouth-logo .

install: build
	./install-desktop.sh

uninstall:
	./uninstall-desktop.sh

run: build
	./dist/plymouth-logo $(ARGS)

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: vet test

fmt:
	$(GO) fmt ./...

clean:
	rm -f -- dist/plymouth-logo
	@if [ -d dist ]; then rmdir --ignore-fail-on-non-empty dist; fi

help:
	@echo 'make              Build dist/plymouth-logo with embedded UI and assets'
	@echo 'make run          Build and launch the app window'
	@echo 'make install      Install the application-menu launcher'
	@echo 'make uninstall    Remove the installed app, launcher, and icons'
	@echo 'make run ARGS="--port 8091 --no-browser"'
	@echo 'make check        Run vet and tests'
	@echo 'make race         Run tests with the race detector'
	@echo 'make fmt          Format Go source files'
	@echo 'make clean        Remove only the built executable'
