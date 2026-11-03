package config

import (
	"strings"
	"testing"
)

func TestNewToken(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		tok := NewToken()
		if len(tok) != TokenLength {
			t.Fatalf("长度 = %d", len(tok))
		}
		if strings.ContainsAny(tok, "0Oo1lI") {
			t.Fatalf("含易混淆字符: %s", tok)
		}
		if strings.Trim(tok, tokenAlphabet) != "" {
			t.Fatalf("含字母表外字符: %s", tok)
		}
		if seen[tok] {
			t.Fatalf("重复: %s", tok)
		}
		seen[tok] = true
	}
	if n := len(tokenAlphabet); n != 56 {
		t.Errorf("字母表长度 = %d", n)
	}
}
