VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")

.PHONY: build clean

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o giff

clean:
	rm -f giff
