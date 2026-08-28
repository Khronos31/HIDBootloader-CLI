// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/binary"
	"testing"
)

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

func TestDecodeBootInfo(t *testing.T) {
	report := make([]byte, 64)
	report[0] = queryDevice
	report[1] = 56
	report[2] = 0x01
	report[3] = 0x01
	binary.LittleEndian.PutUint32(report[4:8], 0x1000)
	binary.LittleEndian.PutUint32(report[8:12], 0x3000)
	report[12] = 0x02
	binary.LittleEndian.PutUint32(report[13:17], 0xf00000)
	binary.LittleEndian.PutUint32(report[17:21], 0x100)
	report[21] = 0xff

	info, err := decodeBootInfo(report)
	if err != nil {
		t.Fatalf("decodeBootInfo: %v", err)
	}
	if info.bytesPerPacket != 56 || info.deviceFamily != 0x01 {
		t.Fatalf("unexpected header: %#v", info)
	}
	if len(info.regions) != 2 {
		t.Fatalf("decoded %d regions, want 2", len(info.regions))
	}
	if info.regions[0].address != 0x1000 || info.regions[0].size != 0x3000 {
		t.Fatalf("unexpected program region: %#v", info.regions[0])
	}
}

func TestDecodeBootInfoAcceptsReportIDZero(t *testing.T) {
	report := make([]byte, 65)
	report[1] = queryDevice
	report[2] = 56
	report[3] = 0x01
	report[12] = 0xff
	if _, err := decodeBootInfo(report); err != nil {
		t.Fatalf("decodeBootInfo with report ID: %v", err)
	}
}
