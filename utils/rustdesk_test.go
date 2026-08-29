package utils

import "testing"

func TestNormalizeRustDeskID(t *testing.T) {
	tests := map[string]string{
		"384 308 369":           "384308369",
		" 384\t308\n369 ":       "384308369",
		"384\u00a0308\u3000369": "384308369",
		"custom-peer_01":        "custom-peer_01",
		"desk\u200b\ufeff01":    "desk01",
	}
	for input, want := range tests {
		if got := NormalizeRustDeskID(input); got != want {
			t.Fatalf("NormalizeRustDeskID(%q) = %q, want %q", input, got, want)
		}
	}
}
