.PHONY: build build-linux-amd64 build-linux-arm64 build-linux-armv7 build-darwin-arm64 build-all test clean \
       deploy deploy-gigantic deploy-pipvs deploy-piforza deploy-mini deploy-FH97NV255C

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

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o build/bin/homelab-agent-darwin-arm64 ./cmd/homelab-agent/

build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7 build-darwin-arm64

deploy-gigantic: build-linux-amd64
	scp build/bin/homelab-agent-linux-amd64 gigantic:~/homelab-agent
	ssh -t gigantic 'sudo systemctl stop homelab-agent; sudo cp ~/homelab-agent /usr/local/bin/homelab-agent && sudo systemctl start homelab-agent'

deploy-pipvs: build-linux-armv7
	scp build/bin/homelab-agent-linux-armv7 pipvs:~/homelab-agent
	ssh -t pipvs 'sudo systemctl stop homelab-agent; sudo cp ~/homelab-agent /usr/local/bin/homelab-agent && sudo systemctl start homelab-agent'

deploy-piforza: build-linux-arm64
	scp build/bin/homelab-agent-linux-arm64 piforza:~/homelab-agent
	ssh -t piforza 'sudo systemctl stop homelab-agent; sudo cp ~/homelab-agent /usr/local/bin/homelab-agent && sudo systemctl start homelab-agent'

deploy-mini: build-darwin-arm64
	scp build/bin/homelab-agent-darwin-arm64 mini:~/homelab-agent
	ssh -t mini 'sudo pkill homelab-agent; sudo cp ~/homelab-agent /usr/local/bin/homelab-agent && sudo /usr/local/bin/homelab-agent &'

deploy-FH97NV255C: build-darwin-arm64
	scp build/bin/homelab-agent-darwin-arm64 FH97NV255C:~/homelab-agent
	ssh -t FH97NV255C 'sudo pkill homelab-agent; sudo cp ~/homelab-agent /usr/local/bin/homelab-agent && sudo /usr/local/bin/homelab-agent &'

deploy: deploy-gigantic deploy-pipvs deploy-piforza deploy-mini deploy-FH97NV255C

test:
	go test ./...

clean:
	rm -rf build/bin
