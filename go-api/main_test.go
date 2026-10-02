package main

import "testing"

func TestListenAddr(t *testing.T) {
	t.Setenv("PORT", "")
	if got := listenAddr(); got != ":8080" {
		t.Errorf("default = %q, want :8080", got)
	}
	t.Setenv("PORT", "10000")
	if got := listenAddr(); got != ":10000" {
		t.Errorf("with PORT = %q, want :10000", got)
	}
}
