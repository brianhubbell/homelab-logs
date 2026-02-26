package goutils

// Message wraps a payload with provenance metadata via a Watermark.
type Message[T any] struct {
	Payload   T          `json:"payload"`
	Watermark *Watermark `json:"watermark"`
}

// NewMessage creates a Message with the given payload and a new Watermark.
// If an existing watermark is provided, it is chained into the new watermark.
func NewMessage[T any](payload T, existing *Watermark, typ string) *Message[T] {
	return &Message[T]{
		Payload:   payload,
		Watermark: NewWatermark(existing, typ),
	}
}
