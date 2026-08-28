// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

package heximage

import (
	"strings"
	"testing"
)

const sampleHEX = ":020000040001F9\n:100000000102030405060708090A0B0C0D0E0F1068\n:00000001FF\n"

func TestParseExtendedLinearAddress(t *testing.T) {
	image, err := Parse(strings.NewReader(sampleHEX))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if image.Len() != 16 {
		t.Fatalf("Len = %d, want 16", image.Len())
	}
	if value, ok := image.Byte(0x00010000); !ok || value != 0x01 {
		t.Fatalf("first byte = %#x, %v", value, ok)
	}
	if value, ok := image.Byte(0x0001000f); !ok || value != 0x10 {
		t.Fatalf("last byte = %#x, %v", value, ok)
	}
}

func TestParseRejectsBadChecksum(t *testing.T) {
	_, err := Parse(strings.NewReader(":0100000000FE\n:00000001FF\n"))
	if err == nil {
		t.Fatal("Parse accepted a bad checksum")
	}
}

func TestParseRejectsConflictingData(t *testing.T) {
	text := ":0100000001FE\n:0100000002FD\n:00000001FF\n"
	if _, err := Parse(strings.NewReader(text)); err == nil {
		t.Fatal("Parse accepted conflicting data")
	}
}

func TestParseRequiresEOF(t *testing.T) {
	_, err := Parse(strings.NewReader(":0100000001FE\n"))
	if err == nil {
		t.Fatal("Parse accepted a file without EOF")
	}
}
