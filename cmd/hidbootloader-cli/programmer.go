// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/Khronos31/HIDBootloader-CLI/internal/heximage"
)

const (
	unlockConfig       = byte(0x03)
	eraseDevice        = byte(0x04)
	programDevice      = byte(0x05)
	programComplete    = byte(0x06)
	getData            = byte(0x07)
	resetCommand       = byte(0x08)
	signFlash          = byte(0x09)
	queryExtendedInfo  = byte(0x0c)
	programMemory      = byte(0x01)
	configMemory       = byte(0x03)
	bootloaderV101Flag = byte(0xa5)
)

type extendedInfo struct {
	bootloaderVersion  uint16
	applicationVersion uint16
	signatureAddress   uint32
	signatureValue     uint16
	erasePageSize      uint32
}

type programSegment struct {
	start uint32
	data  []byte
}

type programmer struct {
	file *os.File
	boot bootInfo
	ext  *extendedInfo
}

func makeCommand(command byte) []byte {
	packet := make([]byte, hidPacketSize)
	packet[1] = command
	return packet
}

func newProgrammer(path string) (*programmer, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &programmer{file: file}, nil
}

func (p *programmer) close() error {
	return p.file.Close()
}

func (p *programmer) send(packet []byte) error {
	return writeReport(p.file, packet)
}

func (p *programmer) receive(command byte) ([]byte, error) {
	report, err := readReport(p.file, 5000*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if len(report) >= 2 && report[0] == 0 && report[1] == command {
		report = report[1:]
	}
	if len(report) == 0 || report[0] != command {
		got := byte(0)
		if len(report) > 0 {
			got = report[0]
		}
		return nil, fmt.Errorf("unexpected response command 0x%02x, expected 0x%02x", got, command)
	}
	return report, nil
}

func (p *programmer) query() error {
	if err := p.send(makeCommand(queryDevice)); err != nil {
		return fmt.Errorf("send QUERY_DEVICE: %w", err)
	}
	report, err := p.receive(queryDevice)
	if err != nil {
		return fmt.Errorf("receive QUERY_DEVICE: %w", err)
	}
	p.boot, err = decodeBootInfo(report)
	if err != nil {
		return err
	}
	if p.boot.versionFlag == bootloaderV101Flag {
		if err := p.queryExtended(); err != nil {
			return err
		}
	}
	return nil
}

func (p *programmer) queryExtended() error {
	if err := p.send(makeCommand(queryExtendedInfo)); err != nil {
		return fmt.Errorf("send QUERY_EXTENDED_INFO: %w", err)
	}
	report, err := p.receive(queryExtendedInfo)
	if err != nil {
		return fmt.Errorf("receive QUERY_EXTENDED_INFO: %w", err)
	}
	if len(report) < 29 {
		return fmt.Errorf("short QUERY_EXTENDED_INFO response: %d bytes", len(report))
	}
	p.ext = &extendedInfo{
		bootloaderVersion:  binary.LittleEndian.Uint16(report[1:3]),
		applicationVersion: binary.LittleEndian.Uint16(report[3:5]),
		signatureAddress:   binary.LittleEndian.Uint32(report[5:9]),
		signatureValue:     binary.LittleEndian.Uint16(report[9:11]),
		erasePageSize:      binary.LittleEndian.Uint32(report[11:15]),
	}
	if p.ext.erasePageSize == 0 {
		return fmt.Errorf("invalid QUERY_EXTENDED_INFO erase page size")
	}
	return nil
}

func (p *programmer) erase() error {
	if err := p.send(makeCommand(eraseDevice)); err != nil {
		return fmt.Errorf("send ERASE_DEVICE: %w", err)
	}
	// ERASE_DEVICE has no response of its own. QUERY_DEVICE is the protocol's
	// completion poll and prevents programming while erase is still active.
	return p.query()
}

func (p *programmer) program(segments []programSegment) error {
	if p.boot.deviceFamily != 0x01 {
		return fmt.Errorf("unsupported device family for the initial writer: %s", familyName(p.boot.deviceFamily))
	}
	for _, segment := range segments {
		if err := p.programSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func (p *programmer) programSegment(segment programSegment) error {
	packetBytes := int(p.boot.bytesPerPacket)
	if packetBytes < 2 || packetBytes > 58 || packetBytes%2 != 0 {
		return fmt.Errorf("unsupported PIC18 packet size: %d", packetBytes)
	}

	pendingData := false
	lastWasComplete := false
	for offset := 0; offset < len(segment.data); {
		count := len(segment.data) - offset
		if count > packetBytes {
			count = packetBytes
		}
		if count%2 != 0 {
			count--
		}
		if count == 0 {
			return fmt.Errorf("odd-sized PIC18 program segment at 0x%08x", segment.start+uint32(offset))
		}

		payload := segment.data[offset : offset+count]
		if allFF(payload) {
			// The bootloader buffers program packets. A blank gap in the HEX
			// image is a discontinuity, so commit the previous buffer before
			// jumping to the next non-blank address.
			if pendingData {
				if err := p.send(makeCommand(programComplete)); err != nil {
					return fmt.Errorf("send PROGRAM_COMPLETE before gap at 0x%08x: %w", segment.start+uint32(offset), err)
				}
				pendingData = false
				lastWasComplete = true
			}
		} else {
			packet := makeCommand(programDevice)
			binary.LittleEndian.PutUint32(packet[2:6], segment.start+uint32(offset))
			packet[6] = byte(count)
			copy(packet[len(packet)-count:], payload)
			if err := p.send(packet); err != nil {
				return fmt.Errorf("program at 0x%08x: %w", segment.start+uint32(offset), err)
			}
			pendingData = true
			lastWasComplete = false
		}
		offset += count
	}
	if !lastWasComplete {
		if err := p.send(makeCommand(programComplete)); err != nil {
			return fmt.Errorf("send PROGRAM_COMPLETE at 0x%08x: %w", segment.start, err)
		}
	}
	return nil
}

func (p *programmer) verify(segments []programSegment, signed bool) error {
	packetBytes := int(p.boot.bytesPerPacket)
	if packetBytes < 2 || packetBytes > 58 || packetBytes%2 != 0 {
		return fmt.Errorf("unsupported PIC18 packet size: %d", packetBytes)
	}
	for _, segment := range segments {
		for offset := 0; offset < len(segment.data); {
			count := len(segment.data) - offset
			if count > packetBytes {
				count = packetBytes
			}
			if count%2 != 0 {
				count--
			}
			if count == 0 {
				return fmt.Errorf("odd-sized PIC18 verify segment at 0x%08x", segment.start+uint32(offset))
			}

			address := segment.start + uint32(offset)
			actual, err := p.readData(address, count)
			if err != nil {
				return err
			}
			for index, value := range actual {
				expected := segment.data[offset+index]
				if signed {
					expected = signedByte(p.ext, address+uint32(index), expected)
				}
				if value != expected {
					return fmt.Errorf("verify failed at 0x%08x: expected 0x%02x, got 0x%02x", address+uint32(index), expected, value)
				}
			}
			offset += count
		}
	}
	return nil
}

func (p *programmer) readData(address uint32, count int) ([]byte, error) {
	packet := makeCommand(getData)
	binary.LittleEndian.PutUint32(packet[2:6], address)
	packet[6] = byte(count)
	if err := p.send(packet); err != nil {
		return nil, fmt.Errorf("get data at 0x%08x: %w", address, err)
	}
	report, err := p.receive(getData)
	if err != nil {
		return nil, fmt.Errorf("receive data at 0x%08x: %w", address, err)
	}
	if len(report) < 6 {
		return nil, fmt.Errorf("short GET_DATA response at 0x%08x", address)
	}
	actualAddress := binary.LittleEndian.Uint32(report[1:5])
	actualCount := int(report[5])
	if actualAddress != address || actualCount != count || actualCount > len(report)-6 {
		return nil, fmt.Errorf("invalid GET_DATA response at 0x%08x: address=0x%08x count=%d", address, actualAddress, actualCount)
	}
	data := make([]byte, actualCount)
	copy(data, report[len(report)-actualCount:])
	return data, nil
}

func (p *programmer) sign() error {
	if err := p.send(makeCommand(signFlash)); err != nil {
		return fmt.Errorf("send SIGN_FLASH: %w", err)
	}
	if err := p.query(); err != nil {
		return fmt.Errorf("poll SIGN_FLASH: %w", err)
	}
	return nil
}

func resetDevice(path string) error {
	programmer, err := newProgrammer(path)
	if err != nil {
		return err
	}
	defer programmer.close()
	if err := programmer.send(makeCommand(resetCommand)); err != nil {
		return err
	}
	fmt.Printf("reset sent to %s\n", path)
	return nil
}

func programImage(path, imagePath string, verifyOnly bool) error {
	image, err := heximage.LoadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read Intel HEX: %w", err)
	}
	programmer, err := newProgrammer(path)
	if err != nil {
		return err
	}
	defer programmer.close()
	if err := programmer.query(); err != nil {
		return err
	}
	if programmer.boot.deviceFamily != 0x01 {
		return fmt.Errorf("unsupported device family: %s", familyName(programmer.boot.deviceFamily))
	}
	segments, ignoredConfig, err := makeProgramSegments(image, programmer.boot)
	if err != nil {
		return err
	}
	if ignoredConfig > 0 {
		fmt.Printf("warning: ignoring %d HEX bytes in configuration memory\n", ignoredConfig)
	}
	if verifyOnly {
		if err := programmer.verify(segments, false); err != nil {
			return err
		}
		fmt.Println("verify successful")
		return nil
	}

	fmt.Println("erasing device")
	if err := programmer.erase(); err != nil {
		return err
	}
	fmt.Println("programming device")
	if err := programmer.program(segments); err != nil {
		return err
	}
	fmt.Println("verifying device")
	if err := programmer.verify(segments, false); err != nil {
		return err
	}
	if programmer.ext != nil {
		fmt.Println("signing flash")
		if err := programmer.sign(); err != nil {
			return err
		}
		if err := programmer.verify(segments, true); err != nil {
			return err
		}
	}
	if err := programmer.send(makeCommand(resetCommand)); err != nil {
		return fmt.Errorf("send RESET_DEVICE: %w", err)
	}
	fmt.Println("write, verify, and reset successful")
	return nil
}

func makeProgramSegments(image *heximage.Image, boot bootInfo) ([]programSegment, int, error) {
	if len(boot.regions) == 0 {
		return nil, 0, fmt.Errorf("bootloader reported no memory regions")
	}
	segments := make([]programSegment, 0)
	for _, region := range boot.regions {
		if region.typ != programMemory {
			continue
		}
		if region.size == 0 || region.size > 64*1024*1024 {
			return nil, 0, fmt.Errorf("unsafe program region: address=0x%08x size=0x%08x", region.address, region.size)
		}
		segments = append(segments, programSegment{start: region.address, data: make([]byte, region.size)})
		for index := range segments[len(segments)-1].data {
			segments[len(segments)-1].data[index] = 0xff
		}
	}
	if len(segments) == 0 {
		return nil, 0, fmt.Errorf("bootloader reported no program memory region")
	}

	ignoredConfig := 0
	for _, address := range image.Addresses() {
		value, _ := image.Byte(address)
		placed := false
		for index := range segments {
			end := uint64(segments[index].start) + uint64(len(segments[index].data))
			if uint64(address) < uint64(segments[index].start) || uint64(address) >= end {
				continue
			}
			segments[index].data[int(uint64(address)-uint64(segments[index].start))] = value
			placed = true
			break
		}
		if placed {
			continue
		}
		if addressInRegion(address, boot.regions, configMemory) {
			ignoredConfig++
			continue
		}
		return nil, 0, fmt.Errorf("HEX data at 0x%08x is outside bootloader program/config regions", address)
	}
	return segments, ignoredConfig, nil
}

func addressInRegion(address uint32, regions []memoryRegion, typ byte) bool {
	for _, region := range regions {
		if region.typ != typ {
			continue
		}
		if uint64(address) >= uint64(region.address) && uint64(address) < uint64(region.address)+uint64(region.size) {
			return true
		}
	}
	return false
}

func allFF(data []byte) bool {
	for _, value := range data {
		if value != 0xff {
			return false
		}
	}
	return true
}

func signedByte(info *extendedInfo, address uint32, fallback byte) byte {
	if info == nil {
		return fallback
	}
	if address == info.signatureAddress {
		return byte(info.signatureValue)
	}
	if address == info.signatureAddress+1 {
		return byte(info.signatureValue >> 8)
	}
	return fallback
}
