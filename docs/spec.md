# Initial specification

## Goal

Provide a Linux CLI that can update firmware on a compatible USB HID
bootloader device, primarily Bit Trade One AD00020/AD00020P.

## Initial commands

- `--list`: enumerate compatible bootloader devices.
- `--write IMAGE.hex`: parse Intel HEX, erase, program, verify, and reset one
  selected device.
- `--verify IMAGE.hex`: compare the image with device memory without erasing or
   programming.
- `--reset`: ask the bootloader to leave boot mode.

The exact command-line spelling may change before the first stable release.

## Acceptance criteria

1. `--help` works without a device and documents safe operation.
2. A bootloader-mode AD00020 is listed with enough information to distinguish
   it from the normal application-mode HID device.
3. A known-good HEX image can be written and verified on an AD00020.
4. A failed or malformed image is rejected before erase is attempted.
5. The release binary runs on the supported Linux target without Qt or a
   separately installed GUI runtime.
6. The source tree contains no proprietary Microchip/BTO source, binary,
   artwork, or credentials.

## Non-goals for the first release

- Windows support.
- Qt GUI.
- Serial/HID configuration of the AD00020 application firmware.
- Replacing or modifying the bootloader firmware.
- Support for arbitrary PIC families before AD00020 compatibility is proven.
