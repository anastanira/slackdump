package chunk

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestPlayer_Thread(t *testing.T) {
	data := marshalChunks(t, testThreads)
	p := Player{
		f: &File{
			rs:  bytes.NewReader(data),
			idx: testThreadsIndex,
		},
		pointer: make(offsets),
	}
	m, err := p.Thread("C1234567890", "1234567890.123456")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m))
	}
	// again
	m, err = p.Thread("C1234567890", "1234567890.123456")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m))
	}
	// should error
	m, err = p.Thread("C1234567890", "1234567890.123456")
	if !errors.Is(err, io.EOF) {
		t.Error(err, "expected io.EOF")
	}
	if len(m) > 0 {
		t.Fatalf("expected 0 messages, got %d", len(m))
	}
}