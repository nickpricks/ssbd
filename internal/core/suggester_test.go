package core

import (
	"strings"
	"testing"
)

func TestSuggest_ShortPassword(t *testing.T) {
	result := Score("abc")
	found := false
	for _, s := range result.Suggestions {
		if strings.Contains(s, "length") {
			found = true
		}
	}
	if !found {
		t.Error("expected length suggestion for short password")
	}
}

func TestSuggest_NoUppercase(t *testing.T) {
	result := Score("abcdefgh1234!@")
	found := false
	for _, s := range result.Suggestions {
		if strings.Contains(s, "uppercase") {
			found = true
		}
	}
	if !found {
		t.Error("expected uppercase suggestion")
	}
}

func TestSuggest_NoSymbols(t *testing.T) {
	result := Score("ABCDefgh1234")
	found := false
	for _, s := range result.Suggestions {
		if strings.Contains(s, "special characters") {
			found = true
		}
	}
	if !found {
		t.Error("expected symbol suggestion")
	}
}

func TestSuggest_CommonPassword(t *testing.T) {
	result := Score("password")
	found := false
	for _, s := range result.Suggestions {
		if strings.Contains(s, "commonly used") {
			found = true
		}
	}
	if !found {
		t.Error("expected common password suggestion")
	}
}

func TestSuggest_StrongPassword(t *testing.T) {
	result := Score("kX9#mQ2!pL7&nR4@wZ8$")
	// Strong password should have minimal or no critical suggestions
	for _, s := range result.Suggestions {
		if strings.Contains(s, "commonly used") || strings.Contains(s, "data breach") {
			t.Errorf("strong password should not get critical suggestion: %s", s)
		}
	}
}
