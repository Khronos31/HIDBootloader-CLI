// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

// Package heximage parses Intel HEX files without depending on a vendor
// programmer library.
package heximage

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Image struct {
	data map[uint32]byte
}

func LoadFile(path string) (*Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

func Parse(reader io.Reader) (*Image, error) {
	image := &Image{data: make(map[uint32]byte)}
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	baseAddress := uint32(0)
	seenEOF := false
	dataRecords := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if seenEOF {
			return nil, fmt.Errorf("line %d: data after EOF record", lineNumber)
		}
		if !strings.HasPrefix(line, ":") {
			return nil, fmt.Errorf("line %d: missing ':'", lineNumber)
		}
		encoded := line[1:]
		if len(encoded)%2 != 0 {
			return nil, fmt.Errorf("line %d: odd number of hexadecimal digits", lineNumber)
		}
		record := make([]byte, len(encoded)/2)
		if _, err := hex.Decode(record, []byte(encoded)); err != nil {
			return nil, fmt.Errorf("line %d: invalid hexadecimal record: %w", lineNumber, err)
		}
		if len(record) < 5 {
			return nil, fmt.Errorf("line %d: record is too short", lineNumber)
		}
		byteCount := int(record[0])
		if len(record) != byteCount+5 {
			return nil, fmt.Errorf("line %d: byte count is %d, record contains %d data bytes", lineNumber, byteCount, len(record)-5)
		}
		var checksum byte
		for _, value := range record {
			checksum += value
		}
		if checksum != 0 {
			return nil, fmt.Errorf("line %d: checksum mismatch", lineNumber)
		}

		address := uint32(record[1])<<8 | uint32(record[2])
		recordType := record[3]
		payload := record[4 : 4+byteCount]
		switch recordType {
		case 0x00:
			dataRecords++
			start := baseAddress + address
			if uint64(start)+uint64(len(payload)) > 0x100000000 {
				return nil, fmt.Errorf("line %d: address overflows 32-bit space", lineNumber)
			}
			for offset, value := range payload {
				location := start + uint32(offset)
				if previous, exists := image.data[location]; exists && previous != value {
					return nil, fmt.Errorf("line %d: conflicting data at address 0x%08x", lineNumber, location)
				}
				image.data[location] = value
			}
		case 0x01:
			if byteCount != 0 || address != 0 {
				return nil, fmt.Errorf("line %d: malformed EOF record", lineNumber)
			}
			seenEOF = true
		case 0x02:
			if byteCount != 2 || address != 0 {
				return nil, fmt.Errorf("line %d: malformed extended segment address record", lineNumber)
			}
			baseAddress = uint32(payload[0])<<12 | uint32(payload[1])<<4
		case 0x04:
			if byteCount != 2 || address != 0 {
				return nil, fmt.Errorf("line %d: malformed extended linear address record", lineNumber)
			}
			baseAddress = uint32(payload[0])<<24 | uint32(payload[1])<<16
		case 0x03, 0x05:
			// Start-address records describe execution entry points. They do not
			// alter the memory image sent to the bootloader.
			if byteCount != 4 || address != 0 {
				return nil, fmt.Errorf("line %d: malformed start-address record", lineNumber)
			}
		default:
			return nil, fmt.Errorf("line %d: unsupported record type 0x%02x", lineNumber, recordType)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !seenEOF {
		return nil, fmt.Errorf("missing EOF record")
	}
	if dataRecords == 0 {
		return nil, fmt.Errorf("HEX file contains no data records")
	}
	return image, nil
}

func (image *Image) Len() int {
	return len(image.data)
}

func (image *Image) Addresses() []uint32 {
	addresses := make([]uint32, 0, len(image.data))
	for address := range image.data {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i] < addresses[j] })
	return addresses
}

func (image *Image) Byte(address uint32) (byte, bool) {
	value, ok := image.data[address]
	return value, ok
}
