# go-utils

Go utility library with logging, string conversion, throttling, and message watermarking. Zero external dependencies — stdlib only.

Go equivalent of [`@brianhubbell/node-utils`](https://github.com/brianhubbell/node-utils).

## Install

This is a private module. Configure Go to bypass the public proxy:

```bash
go env -w GOPRIVATE=github.com/brianhubbell/*
```

Then install:

```bash
go get github.com/brianhubbell/go-utils
```

## Usage

### StrToBool

Converts a string to a boolean. Returns `true` for `"true"`, `"1"`, `"yes"` (case-insensitive, trimmed).

```go
goutils.StrToBool("yes")   // true
goutils.StrToBool("false") // false
```

### Log / Err / Debug

Structured logging via `slog`. Set the `DEBUG` env var to `true`/`1`/`yes` to enable debug output.

```go
goutils.Log("request received", "method", "GET", "path", "/api")
goutils.Err("request failed", "status", 500)
goutils.Debug("cache miss", "key", "user:42")
```

### Throttle

Rate-limits function execution per key.

```go
t := goutils.NewThrottle(func(key string) {
    fmt.Println("executing", key)
}, 5*time.Second)

t.Exec("user:1") // true — executes
t.Exec("user:1") // false — throttled
t.Exec("user:2") // true — different key
```

### Watermark

Provenance metadata with optional chaining. Reads `APP_NAME` and `APP_VERSION` env vars.

```go
w := goutils.NewWatermark(nil, "ingest")
// chain onto an existing watermark
w2 := goutils.NewWatermark(w, "transform")
```

### Message

Generic message wrapper that automatically creates a watermark.

```go
msg := goutils.NewMessage("hello", nil, "greeting")
// msg.Payload == "hello"
// msg.Watermark.Type == "greeting"

// works with any type
msg2 := goutils.NewMessage(Order{ID: 42, Total: 9.99}, nil, "order")
```

## Test

```bash
go test ./... -v
```
