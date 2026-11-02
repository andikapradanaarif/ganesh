.PHONY: build build-collector

# A smaller artifact; capture headroom still needs installation-specific evidence.
build:
	go build -ldflags='-s -w' -o bin/ganesh ./cmd/ganesh

# Experimental capture-only entry point; does not install or enable hooks.
build-collector:
	go build -ldflags='-s -w' -o bin/ganesh-capture ./cmd/ganesh-capture
