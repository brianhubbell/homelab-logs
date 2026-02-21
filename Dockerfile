FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.Version=${VERSION}" -o /homelab-agent ./cmd/homelab-agent/

FROM alpine:3.20
COPY --from=build /homelab-agent /usr/local/bin/homelab-agent
ENTRYPOINT ["homelab-agent"]
