package main

import "testing"

// TestPlaceholder proves the Go module builds and tests with no environment
// variables set (S0 acceptance criterion 4). Replace with real coverage as
// slices land.
func TestPlaceholder(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("arithmetic is broken")
	}
}
