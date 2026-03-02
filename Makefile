.PHONY: build build-linux-amd64 build-linux-arm64 build-linux-armv7 build-all test clean \
       deploy deploy-gigantic deploy-pipvs deploy-piforza

VERSION ?= $(shell git describe --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o build/bin/homelab-agent ./cmd/homelab-agent/

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/bin/homelab-agent-linux-amd64 ./cmd/homelab-agent/

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/bin/homelab-agent-linux-arm64 ./cmd/homelab-agent/

build-linux-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o build/bin/homelab-agent-linux-armv7 ./cmd/homelab-agent/

build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7

deploy-gigantic: build-linux-amd64
	scp build/bin/homelab-agent-linux-amd64 gigantic:/opt/homelab-services/homelab-agent/build/bin/homelab-agent
	ssh gigantic 'sudo systemctl restart homelab-agent'

deploy-pipvs: build-linux-armv7
	scp build/bin/homelab-agent-linux-armv7 pipvs:/opt/homelab-services/homelab-agent/build/bin/homelab-agent
	ssh pipvs 'sudo systemctl restart homelab-agent'

deploy-piforza: build-linux-arm64
	scp build/bin/homelab-agent-linux-arm64 piforza:/opt/homelab-services/homelab-agent/build/bin/homelab-agent
	ssh piforza 'sudo systemctl restart homelab-agent'

deploy: deploy-gigantic deploy-pipvs deploy-piforza

test:
	go test ./...

clean:
	rm -rf build/bin
