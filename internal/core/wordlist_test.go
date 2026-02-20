package core

import "testing"

func TestLoadWordlist(t *testing.T) {
	words := LoadWordlist()
	if len(words) == 0 {
		t.Fatal("wordlist is empty")
	}
	// EFF large wordlist has 7776 words
	if len(words) != 7776 {
		t.Errorf("expected 7776 words, got %d", len(words))
	}
	// Spot check a few known words
	found := false
	for _, w := range words {
		if w == "abandon" || w == "zoom" || w == "abacus" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find known EFF wordlist entries")
	}
}

func TestLoadWordlist_Idempotent(t *testing.T) {
	w1 := LoadWordlist()
	w2 := LoadWordlist()
	if len(w1) != len(w2) {
		t.Error("wordlist should be loaded once and cached")
	}
}
