# License audit

Date: 2026-08-28

## Conclusion

The public repository must not be a source-code fork of the existing Qt
HIDBootloader implementation. The initial implementation therefore uses a
source-independent implementation policy: only newly written code is
committed here, and no existing source is copied or mechanically translated.

## Audited material

### Bit Trade One distribution

The AD00020 public repository says that its `HIDBootLoader.exe` is the same
software as the one in Microchip's MCHPFSUSB library. Its root `LICENSE.md`
points to Bit Trade One's Assembly Desk License, but that does not by itself
relicense Microchip-owned code contained in the referenced tool.

- https://github.com/bit-trade-one/AD00020-USB_IR_Remote_Controller
- https://bit-trade-one.co.jp/adl/

### Microchip MCHPFSUSB

The MCHPFSUSB license is a proprietary, limited license. It limits use to
Microchip devices/products and places conditions on disclosure, sublicensing,
and distribution. The installed Qt source also carries Microchip notices with
the same product-use restriction. It is not suitable as the license for this
repository's source code.

- https://documentation.help/MCHPFSUSB/MCHPFSUSB.html
- https://www.microchip.com/en-us/about/license-agreement-end-user

### HIDAPI

The historical source distribution contains HIDAPI with selectable GPL, BSD,
or original HIDAPI terms. The historical copy and its headers are not copied
into this repository. If a future version uses HIDAPI, the exact upstream
version and applicable license files must be recorded here before distribution.

## Implementation rule

Do not copy or mechanically translate Microchip/BTO source, Qt UI code,
Microchip images, or their binaries. Protocol constants and device behavior
may be documented as compatibility information, but implementation code must
be written independently and must not expose or depend on proprietary source
files. This policy reduces risk; it is not a legal determination that every
possible reimplementation is non-infringing.

This is a project engineering decision, not legal advice. A commercial/public
release involving redistributed Microchip materials should be reviewed by a
qualified lawyer or cleared with Microchip.
