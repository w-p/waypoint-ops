package ui

import (
	"testing"
	"time"
)

func TestSpan(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{3*time.Minute + 32*time.Second, "3m 32s"},
		{41 * time.Minute, "41m"},
		{17*time.Hour + 12*time.Minute, "17h 12m"},
		{50 * time.Hour, "2d 02h"},
		{-time.Second, "0s"},
	}
	for _, tt := range tests {
		if got := span(tt.in); got != tt.want {
			t.Errorf("span(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{999, "999"},
		{530_475, "530k"},
		{5_250_000, "5.2M"},
		{693_000_000, "693M"},
	}
	for _, tt := range tests {
		if got := count(tt.in); got != tt.want {
			t.Errorf("count(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShortModel(t *testing.T) {
	tests := []struct{ in, want string }{
		{"claude-opus-5-5", "opus-5.5"},
		{"claude-fable-5-2", "fable-5.2"},
		{"gpt-x", "gpt-x"},
		{"", "—"},
	}
	for _, tt := range tests {
		if got := shortModel(tt.in); got != tt.want {
			t.Errorf("shortModel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
