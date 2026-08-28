// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

// This file is an independent Linux hidraw implementation. It does not use
// or derive from the Microchip/BTO GUI sources.
package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/Khronos31/HIDBootloader-CLI/internal/heximage"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	defaultVID    = uint16(0x04d8)
	defaultPID    = uint16(0x003c)
	queryDevice   = byte(0x02)
	hidPacketSize = 65

	// Linux HIDRAW_GET_RAW_INFO is _IOR('H', 0x03, struct hidraw_devinfo).
	// struct hidraw_devinfo is 8 bytes on the supported Linux architectures.
	hidrawGetRawInfo = uintptr(0x80084803)
)

var version = "development"

type hidrawDevInfo struct {
	busType uint32
	vendor  uint16
	product uint16
}

type hidDevice struct {
	path    string
	busType uint32
	vendor  uint16
	product uint16
}

type memoryRegion struct {
	typ     byte
	address uint32
	size    uint32
}

type bootInfo struct {
	bytesPerPacket byte
	deviceFamily   byte
	regions        []memoryRegion
}

func main() {
	list := flag.Bool("list", false, "list compatible bootloader HID devices")
	all := flag.Bool("all", false, "list all hidraw devices")
	info := flag.Bool("info", false, "query a compatible bootloader without changing memory")
	path := flag.String("path", "", "specific /dev/hidraw path for --info")
	checkHex := flag.String("check-hex", "", "validate an Intel HEX file without accessing USB")
	vidText := flag.String("vid", fmt.Sprintf("0x%04x", defaultVID), "USB vendor ID")
	pidText := flag.String("pid", fmt.Sprintf("0x%04x", defaultPID), "USB product ID")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("hidbootloader-cli " + version)
		return
	}
	if *checkHex != "" {
		image, err := heximage.LoadFile(*checkHex)
		if err != nil {
			fatal("checking %s: %v", *checkHex, err)
		}
		fmt.Printf("%s: valid Intel HEX (%d data bytes)\n", *checkHex, image.Len())
		return
	}

	if !*list && !*all && !*info {
		usage()
		os.Exit(2)
	}

	vid, err := parseID(*vidText)
	if err != nil {
		fatal("invalid --vid: %v", err)
	}
	pid, err := parseID(*pidText)
	if err != nil {
		fatal("invalid --pid: %v", err)
	}
	if *info && *path != "" {
		device, err := inspectHIDRaw(*path)
		if err != nil {
			fatal("opening %s: %v", *path, err)
		}
		if device.vendor != vid || device.product != pid {
			fatal("%s is VID=%04x PID=%04x, expected VID=%04x PID=%04x", *path, device.vendor, device.product, vid, pid)
		}
		if err := queryDeviceInfo(device.path); err != nil {
			fatal("querying %s: %v", device.path, err)
		}
		return
	}

	devices, err := enumerateHIDRaw()
	if err != nil {
		fatal("enumerating /dev/hidraw*: %v", err)
	}

	found := 0
	for _, device := range devices {
		if !*all && (device.vendor != vid || device.product != pid) {
			continue
		}
		fmt.Printf("%s VID=%04x PID=%04x bus=%d\n", device.path, device.vendor, device.product, device.busType)
		found++
	}

	if found == 0 {
		if *all {
			fmt.Println("no hidraw devices found")
		} else {
			fmt.Printf("no matching bootloader device found (VID=%04x PID=%04x)\n", vid, pid)
		}
	}

	if *info {
		device, err := selectDevice(devices, *path, vid, pid)
		if err != nil {
			fatal("selecting device: %v", err)
		}
		if err := queryDeviceInfo(device.path); err != nil {
			fatal("querying %s: %v", device.path, err)
		}
	}
}

func usage() {
	const text = `Usage:
  hidbootloader-cli --list [--vid 0x04d8] [--pid 0x003c]
  hidbootloader-cli --all
  hidbootloader-cli --info [--path /dev/hidrawX]
  hidbootloader-cli --check-hex IMAGE.hex

The default filter is the Microchip HID bootloader VID/PID. Programming
commands are not implemented yet; this version enumerates devices and can
query bootloader information without changing memory.
`
	fmt.Fprint(flag.CommandLine.Output(), text)
	flag.PrintDefaults()
}

func selectDevice(devices []hidDevice, path string, vendor, product uint16) (hidDevice, error) {
	if path != "" {
		for _, device := range devices {
			if device.path != path {
				continue
			}
			if device.vendor != vendor || device.product != product {
				return hidDevice{}, fmt.Errorf("%s is VID=%04x PID=%04x, expected VID=%04x PID=%04x", path, device.vendor, device.product, vendor, product)
			}
			return device, nil
		}
		return hidDevice{}, fmt.Errorf("%s was not found", path)
	}

	var match *hidDevice
	for index := range devices {
		if devices[index].vendor != vendor || devices[index].product != product {
			continue
		}
		if match != nil {
			return hidDevice{}, fmt.Errorf("multiple matching devices; specify --path")
		}
		match = &devices[index]
	}
	if match == nil {
		return hidDevice{}, fmt.Errorf("no matching bootloader device found (VID=%04x PID=%04x)", vendor, product)
	}
	return *match, nil
}

func queryDeviceInfo(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	request := make([]byte, hidPacketSize)
	request[1] = queryDevice
	if err := writeReport(file, request); err != nil {
		return fmt.Errorf("send QUERY_DEVICE: %w", err)
	}
	report, err := readReport(file, 5000*time.Millisecond)
	if err != nil {
		return fmt.Errorf("receive QUERY_DEVICE: %w", err)
	}
	info, err := decodeBootInfo(report)
	if err != nil {
		return err
	}

	fmt.Printf("%s bootloader: family=%s bytes-per-packet=%d\n", path, familyName(info.deviceFamily), info.bytesPerPacket)
	for _, region := range info.regions {
		fmt.Printf("  region type=%s address=0x%08x size=0x%08x\n", regionName(region.typ), region.address, region.size)
	}
	return nil
}

func writeReport(file *os.File, report []byte) error {
	count, err := file.Write(report)
	if err != nil {
		return err
	}
	if count != len(report) {
		return fmt.Errorf("short HID write: %d of %d bytes", count, len(report))
	}
	return nil
}

func readReport(file *os.File, timeout time.Duration) ([]byte, error) {
	pollFD := struct {
		fd      int32
		events  int16
		revents int16
	}{fd: int32(file.Fd()), events: 0x0001}
	milliseconds := int(timeout / time.Millisecond)
	result, _, errno := syscall.Syscall6(syscall.SYS_POLL, uintptr(unsafe.Pointer(&pollFD)), 1, uintptr(milliseconds), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	if result == 0 {
		return nil, fmt.Errorf("timeout after %s", timeout)
	}
	if pollFD.revents&0x0001 == 0 {
		return nil, fmt.Errorf("HID read returned poll events 0x%x", pollFD.revents)
	}
	report := make([]byte, hidPacketSize)
	count, err := file.Read(report)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("empty HID report")
	}
	return report[:count], nil
}

func decodeBootInfo(report []byte) (bootInfo, error) {
	if len(report) >= 2 && report[0] == 0 && report[1] == queryDevice {
		report = report[1:]
	}
	if len(report) < 58 {
		return bootInfo{}, fmt.Errorf("short QUERY_DEVICE response: %d bytes", len(report))
	}
	if report[0] != queryDevice {
		return bootInfo{}, fmt.Errorf("unexpected QUERY_DEVICE response command: 0x%02x", report[0])
	}

	info := bootInfo{bytesPerPacket: report[1], deviceFamily: report[2]}
	for offset := 3; offset+9 <= len(report) && len(info.regions) < 6; offset += 9 {
		region := memoryRegion{
			typ:     report[offset],
			address: binary.LittleEndian.Uint32(report[offset+1 : offset+5]),
			size:    binary.LittleEndian.Uint32(report[offset+5 : offset+9]),
		}
		if region.typ == 0xff {
			break
		}
		if region.typ != 0 {
			info.regions = append(info.regions, region)
		}
	}
	if info.bytesPerPacket == 0 || info.bytesPerPacket > 58 {
		return bootInfo{}, fmt.Errorf("invalid bootloader packet payload size: %d", info.bytesPerPacket)
	}
	return info, nil
}

func familyName(family byte) string {
	switch family {
	case 0x01:
		return "PIC18"
	case 0x02:
		return "PIC24/dsPIC"
	case 0x03:
		return "PIC32"
	case 0x04:
		return "PIC16"
	default:
		return fmt.Sprintf("unknown(0x%02x)", family)
	}
}

func regionName(region byte) string {
	switch region {
	case 0x01:
		return "program"
	case 0x02:
		return "EEPROM"
	case 0x03:
		return "config"
	case 0x04:
		return "user-ID"
	default:
		return fmt.Sprintf("unknown(0x%02x)", region)
	}
}

func parseID(text string) (uint16, error) {
	text = strings.TrimSpace(text)
	base := 10
	if strings.HasPrefix(strings.ToLower(text), "0x") {
		base = 0
	} else if strings.ContainsAny(strings.ToLower(text), "abcdef") {
		base = 16
	}
	value, err := strconv.ParseUint(text, base, 16)
	if err != nil {
		return 0, err
	}
	return uint16(value), nil
}

func enumerateHIDRaw() ([]hidDevice, error) {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "hidraw") {
			paths = append(paths, filepath.Join("/dev", entry.Name()))
		}
	}
	sort.Strings(paths)

	devices := make([]hidDevice, 0, len(paths))
	for _, path := range paths {
		device, err := inspectHIDRaw(path)
		if err != nil {
			// Devices can disappear during enumeration. Keep listing the others.
			if errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENOENT) {
				continue
			}
			if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: permission denied\n", path)
				continue
			}
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func inspectHIDRaw(path string) (hidDevice, error) {
	file, err := os.Open(path)
	if err != nil {
		return hidDevice{}, err
	}
	defer file.Close()

	info := hidrawDevInfo{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), hidrawGetRawInfo, uintptr(unsafe.Pointer(&info)))
	if errno != 0 {
		return hidDevice{}, errno
	}
	return hidDevice{path: path, busType: info.busType, vendor: info.vendor, product: info.product}, nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
