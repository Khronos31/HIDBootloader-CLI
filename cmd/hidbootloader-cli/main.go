// Copyright (c) 2026 Khronos31
// SPDX-License-Identifier: MIT

// This file is an independent Linux hidraw implementation. It does not use
// or derive from the Microchip/BTO GUI sources.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	defaultVID = uint16(0x04d8)
	defaultPID = uint16(0x003c)

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

func main() {
	list := flag.Bool("list", false, "list compatible bootloader HID devices")
	all := flag.Bool("all", false, "list all hidraw devices")
	vidText := flag.String("vid", fmt.Sprintf("0x%04x", defaultVID), "USB vendor ID")
	pidText := flag.String("pid", fmt.Sprintf("0x%04x", defaultPID), "USB product ID")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("hidbootloader-cli " + version)
		return
	}

	if !*list && !*all {
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
}

func usage() {
	const text = `Usage:
  hidbootloader-cli --list [--vid 0x04d8] [--pid 0x003c]
  hidbootloader-cli --all

The default filter is the Microchip HID bootloader VID/PID. Programming
commands are not implemented yet; this version only enumerates hidraw devices.
`
	fmt.Fprint(flag.CommandLine.Output(), text)
	flag.PrintDefaults()
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
				return nil, fmt.Errorf("%s: permission denied (run as root or install a udev rule)", path)
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
