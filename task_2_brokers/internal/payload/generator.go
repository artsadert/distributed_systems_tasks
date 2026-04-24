package payload

import (
	"crypto/rand"
	"time"
)

type Generator struct {
	size int
}

func NewGenerator(size int) *Generator {
	minSize := len(time.RFC3339Nano) + 1 + 1
	if size < minSize {
		size = minSize
	}
	return &Generator{size: size}
}

func (g *Generator) Generate() []byte {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	prefix := []byte(ts + "|")
	payload := make([]byte, g.size)
	copy(payload, prefix)
	if g.size > len(prefix) {
		_, _ = rand.Read(payload[len(prefix):])
	}
	return payload
}