package model

import "testing"

func TestConfidenceBand(t *testing.T) {
	cases := []struct {
		name string
		in   Confidence
		want string
	}{
		{"high", 0.85, "high"},
		{"medium", 0.6, "medium"},
		{"low", 0.3, "low"},
		{"floor", 0.0, "low"},
		{"ceil", 1.0, "high"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Band(); got != c.want {
				t.Fatalf("Band()=%q want %q", got, c.want)
			}
		})
	}
}
