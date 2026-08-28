// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

package main

import "testing"

func TestParseID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint16
	}{
		{name: "hex", input: "0x04d8", want: 0x04d8},
		{name: "bare hex", input: "04d8", want: 0x04d8},
		{name: "decimal", input: "987", want: 987},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseID(test.input)
			if err != nil {
				t.Fatalf("parseID(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("parseID(%q) = %#x, want %#x", test.input, got, test.want)
			}
		})
	}
}

func TestParseIDRejectsOverflow(t *testing.T) {
	if _, err := parseID("0x10000"); err == nil {
		t.Fatal("parseID accepted a value wider than USB IDs")
	}
}

