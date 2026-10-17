package main

import (
	"bytes"
	"testing"
)

func TestRunVerifiesAndRejects(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected output")
	}
}
