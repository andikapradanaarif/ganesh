.PHONY: build

# A smaller artifact; capture headroom still needs installation-specific evidence.
build:
	go build -ldflags='-s -w' -o bin/ganesh ./cmd/ganesh
