package cmd

import (
	"bytes"
	"testing"
)

func TestOutStyledWithPrefix(t *testing.T) {
	var buf bytes.Buffer
	outStyledWithPrefix(&buf, ansiBlue, "🚀", "  ", "Pushed %s", "branch")

	got := buf.String()
	want := "  🚀 Pushed branch\n"
	if got != want {
		t.Fatalf("outStyledWithPrefix() = %q, want %q", got, want)
	}
}

func TestOutStyledWithoutPrefix(t *testing.T) {
	var buf bytes.Buffer
	outStyled(&buf, ansiGreen, "✅", "Done")

	got := buf.String()
	want := "✅ Done\n"
	if got != want {
		t.Fatalf("outStyled() = %q, want %q", got, want)
	}
}
