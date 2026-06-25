package main

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestReadRawLineFiltersEscapeSequences(t *testing.T) {
	// A leaked cursor-position (DSR) response and an OSC color response prepended
	// to the typed input must be discarded, leaving just the value (TK-162).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = w.Write([]byte("\x1b[24;1R\x1b]11;rgb:0000/0000/0000\x07admin\r"))
		_ = w.Close()
	}()
	got, err := readRawLine(r, io.Discard, false)
	if err != nil {
		t.Fatalf("readRawLine: %v", err)
	}
	if got != "admin" {
		t.Fatalf("got %q, want %q", got, "admin")
	}
}

func TestReadRawLineCtrlCInterrupts(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = w.Write([]byte{3}) // Ctrl-C
		_ = w.Close()
	}()
	if _, err := readRawLine(r, io.Discard, false); !errors.Is(err, errPromptInterrupted) {
		t.Fatalf("want errPromptInterrupted, got %v", err)
	}
}
