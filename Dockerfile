FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -mod=vendor -ldflags "-X main.Version=${VERSION}" -o /homelab-logs ./cmd/homelab-logs/

FROM alpine:3.20
COPY --from=build /homelab-logs /usr/local/bin/homelab-logs
ENTRYPOINT ["homelab-logs"]
