# Protocol facts

This file has each Dialect fact that the code uses, with its source and its status.

Status:
- **VERIFIED**: the fact is in the cited public source. The Builder read the source on 2026-11-10.
- **UNVERIFIED**: the fact has no source, or the source is a family sample or a Builder inference.

NOTE: All facts are UNVERIFIED for the Reference UPS (Tunçmatik Newtech Pro II 1 kVA) until the Owner Capture confirms them.

Source keys (refer to `docs/references.md`):

| Key | Source |
|-----|--------|
| M | Megatec protocol document, published by Network UPS Tools: <https://networkupstools.org/protocols/megatec.html> |
| A | Source A, family sample (Tunçmatik Newtech Pro 1 kVA, 2010): <https://lists.alioth.debian.org/pipermail/nut-upsuser/2010-February/005856.html> |
| B | Source B, standby UPS, Q1 over USB: <https://gist.github.com/ao-kenji/7070940> |
| IS2 | VLT-IS-002 Revision 00 (decisions and test vectors) |
| K1 | Linux kernel documentation, hidraw: <https://docs.kernel.org/hid/hidraw.html> |
| K2 | Linux kernel documentation, introduction to HID report descriptors: <https://docs.kernel.org/hid/hidintro.html> |
| W1 | Microsoft documentation, HIDP_CAPS: <https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/hidpi/ns-hidpi-_hidp_caps> |

## 1. General frame

| No. | Fact | Code | Source | Status |
|-----|------|------|--------|--------|
| P-001 | The computer sends a query followed by CR. The UPS answers with data followed by CR. | `dialect.EncodeRead`, `dialect.frame` | M | VERIFIED |
| P-002 | Fields in a response are separated by one space character. | `dialect.splitFields` | M | VERIFIED |
| P-003 | A UPS that cannot handle a command echoes the command back. | `dialect.isNotSupported` | M | VERIFIED |
| P-004 | A field that is not available or not supported is filled with "@". | `dialect.noValue` | M | VERIFIED |
| P-005 | Fields with only "-" or "#" characters have no value (example "---.-" in QGS). | `dialect.noValue` | A (sample), IS2 | UNVERIFIED |
| P-006 | A rejected command gets the response "(NAK" (example: "Q"). | `dialect.isNotSupported` | A | UNVERIFIED |
| P-007 | The plain response "NAK" (without "(") means not supported. | `dialect.isNotSupported` | IS2 | UNVERIFIED |
| P-008 | Maximum response length is 256 bytes. | `dialect.MaxResponseLen` | IS2 (VLT rule, not a protocol fact) | VERIFIED (rule) |
| P-009 | Responses are printable ASCII. | `dialect.frame` | M (format examples) | UNVERIFIED |
| P-010 | Status and data responses start with "("; F and I responses start with "#". | `dialect.frame` | M | VERIFIED |

## 2. Commands

| No. | Fact | Code | Source | Status |
|-----|------|------|--------|--------|
| P-020 | "Q1" is the status inquiry. | registry | M | VERIFIED |
| P-021 | "D" is the old status inquiry and is disabled (revision 2.7). VLT never sends "D". It is in no class. | registry (no row) | M, IS2 QA-01 | VERIFIED |
| P-022 | "Q" toggles the UPS beeper. It is a CONTROL command. | registry | M, IS2 QA-01 | VERIFIED |
| P-023 | "T", "TL", "Tnn" start battery tests. "CT" cancels a test. | registry (CONTROL) | M, A, VLT-IS-001 03-20-00 | VERIFIED (M), UNVERIFIED (CT, A only) |
| P-024 | "Snn" and "SnnRmmmm" shut down the UPS output (and restart). "C" and "CS" cancel a shutdown. | registry (CONTROL) | M, A | VERIFIED (M), UNVERIFIED (CS, A only) |
| P-025 | "SOFF", "SON", "BZOFF", "BZON" turn the load or the beeper off and on. | registry (CONTROL) | A | UNVERIFIED |
| P-026 | "PEx" and "PDx" enable and disable UPS option flags. They are CONFIG commands. | registry (CONFIG) | A | UNVERIFIED |
| P-027 | "F" is the rating information query. "I" is the UPS information query. | registry | M | VERIFIED |
| P-028 | "QS", "QGS", "QBV", "QWS", "QFLAG", "QRI", "QID", "QVFW", "QMD", "QMOD" are queries in an extended Dialect. | registry | A; QMD, QMOD, QRI, QVFW also in M (21st century chapter) | UNVERIFIED |
| P-029 | The Dialect assignment of each READ command in the registry (Q1, Voltronic-QS, Voltronic). | registry `Dialects` | Builder inference from A and M | UNVERIFIED |

NOTE: vltd polls in two groups (later instruction). Fast group: 2 commands or fewer, every 2 seconds. Slow group: all other READ commands, every 30 seconds. Source: VLT-IS-003 01-30-00 (VLT-IS-001 Revision 02, 03-20-00).

## 3. Q1 and QS response

Format: `(MMM.M NNN.N PPP.P QQQ RR.R S.SS|SS.S TT.T b7b6b5b4b3b2b1b0`

| No. | Field | Width / format | Unit | Source | Status |
|-----|-------|----------------|------|--------|--------|
| P-040 | 1 Input voltage | ddd.d | V | M | VERIFIED |
| P-041 | 2 Input fault voltage | ddd.d | V | M | VERIFIED |
| P-042 | 3 Output voltage | ddd.d | V | M | VERIFIED |
| P-043 | 4 Output current, percent of maximum | ddd | % | M | VERIFIED |
| P-044 | 5 Input frequency | dd.d | Hz | M | VERIFIED |
| P-045 | 6 Battery voltage: per cell "S.SS" (on-line UPS) or total "SS.S" (standby UPS). The parser keeps the value as received and records the form. | d.dd or dd.d | V | M, B | VERIFIED |
| P-046 | 7 Temperature | dd.d | °C | M | VERIFIED |
| P-047 | 8 Status: 8 characters "0" or "1"; the first character is bit 7. | 8 bits | — | M | VERIFIED |
| P-048 | Bit 7 utility fail, bit 6 battery low, bit 5 bypass/boost or buck active, bit 4 UPS failed, bit 3 UPS type standby (0 = on-line), bit 2 test in progress, bit 1 shutdown active, bit 0 beeper on. | `Q1Flags` | M | VERIFIED |
| P-049 | QS has the same 8 fields as Q1. Field 6 is a total battery voltage ("41.8"). | `ParseQS` | A | UNVERIFIED |
| P-050 | The named Q1 bits also apply to QS. | `ParseQS` | IS2 03-00-00-005 4e | UNVERIFIED |
| P-051 | Source B: bit 3 = 1 means a standby UPS, and field 6 is the total voltage. | test | B, IS2 | VERIFIED |

## 4. QGS response (Voltronic)

Format (sample): `(209.7 50.0 219.7 50.0 000.8 019 374.9 374.3 041.8 ---.- 023.3 100000000001`

| No. | Field | Format | Unit | Source | Status |
|-----|-------|--------|------|--------|--------|
| P-060 | 1 Input voltage | ddd.d | V | A | UNVERIFIED |
| P-061 | 2 Input frequency | dd.d | Hz | A | UNVERIFIED |
| P-062 | 3 Output voltage | ddd.d | V | A | UNVERIFIED |
| P-063 | 4 Output frequency | dd.d | Hz | A | UNVERIFIED |
| P-064 | 5 Output current | ddd.d | A | A | UNVERIFIED |
| P-065 | 6 Load | ddd | % | A | UNVERIFIED |
| P-066 | 7, 8 and 10: unknown. Kept as text. | ddd.d | — | A ("unknown1..3") | UNVERIFIED |
| P-067 | 9 Battery voltage | ddd.d | V | A | UNVERIFIED |
| P-068 | 11 Temperature | ddd.d | °C | A | UNVERIFIED |
| P-069 | 12 Status: 12 bits, kept raw. Meaning UNVERIFIED. | 12 bits | — | A, IS2 | UNVERIFIED |

## 5. Other responses

| No. | Command | Fact | Unit | Source | Status |
|-----|---------|------|------|--------|--------|
| P-080 | QBV | 5 fields: battery voltage (ddd.d), number of battery packs in series (dd), number in parallel (dd), battery charge (ddd), runtime estimate (ddd). | V, —, —, %, minutes | A; similar QBL in M | UNVERIFIED |
| P-081 | QMOD | One mode letter. "L" = on AC power, "B" = on battery. The parser keeps the letter raw. | — | A | UNVERIFIED |
| P-082 | QWS | 64 characters "0" or "1". Kept raw. Meaning UNVERIFIED. | — | A | UNVERIFIED |
| P-083 | QFLAG | Letters after "E" are enabled, letters after "D" are disabled. Letter meanings are not used. | — | A | UNVERIFIED |
| P-084 | QRI | 4 fields: rating voltage (ddd.d), rating current (ddd), battery voltage (ddd.d or dd.dd), frequency (dd.d). | V, A, V, Hz | A | UNVERIFIED |
| P-085 | F | `#MMM.M QQQ SS.SS RR.R`: rating voltage, rating current, battery voltage ("SS.SS or SSS.S"), frequency. | V, A, V, Hz | M | VERIFIED |
| P-086 | F | The unit of the rating current is ampere. | A | A ("Current") | UNVERIFIED |
| P-087 | I | `#Company_Name UPS_Model Version`: widths 15, 10 and 10 characters, space between fields, spaces fill short values. | — | M | VERIFIED |
| P-088 | I | The exact spaces of the Reference UPS response are not known. No vector. | — | IS2 | UNVERIFIED |
| P-089 | QID | UPS serial number. Personal data: vlt-probe redacts it. Maximum length accepted by VLT: 32 characters (VLT rule). | — | A, IS2 | UNVERIFIED |
| P-090 | QVFW | `VERFW:` then the firmware version. | — | A | UNVERIFIED |
| P-091 | QMD | 8 fields: model (15, "#" padding), rated VA (7, "#" padding), power factor (dd), phases ("d/d"), nominal input voltage (ddd), nominal output voltage (ddd), number of batteries (dd), battery voltage (dd.d). | —, VA, %, —, V, V, —, V | A (sample only, the source says "What is this ?") | UNVERIFIED |
| P-092 | QMD | Leading "#" characters are padding. A field with only "#" has no value. | — | A, IS2 | UNVERIFIED |

## 6. USB HID

| No. | Fact | Code | Source | Status |
|-----|------|------|--------|--------|
| P-100 | The Reference UPS USB interface has VID 0665 and PID 5161. | `transport.VendorID`, `ProductID` | IS2 | UNVERIFIED |
| P-101 | hidraw write(): the first byte is the report number, 0 if the device does not use numbered reports. | `hidPort.Write`, `hidraw.WriteReport` | K1 | VERIFIED |
| P-102 | hidraw read(): with numbered reports, the first byte is the report number; without, the data starts at the first byte. | `hidraw.ReadReport` | K1 | VERIFIED |
| P-103 | The report descriptor is readable from sysfs (`.../report_descriptor`). | `listHIDIn` | K2 | VERIFIED |
| P-104 | sysfs `uevent` has `HID_ID=<bus>:<vid>:<pid>` (hexadecimal), `HID_NAME` and `HID_UNIQ`. | `readUevent` | Linux kernel source (not read in this session) | UNVERIFIED |
| P-105 | Report descriptor short items: prefix byte with size (bits 0-1, value 3 = 4 bytes), type (bits 2-3) and tag (bits 4-7). Global tags: Report Size 7, Report ID 8, Report Count 9, Push 10, Pop 11. Main tags: Input 8, Output 9, Feature 11. Long item prefix 0xFE. | `ParseReportDescriptor` | USB HID specification 1.11 (not read in this session); examples in K2 | UNVERIFIED |
| P-106 | HidP_GetCaps report lengths include the report ID byte. The ID is zero if report IDs are not used. | `queryHID` | W1 | VERIFIED |
| P-107 | Requests are sent as output reports (not feature reports). | `hidPort` | none | UNVERIFIED (IS2 03-00-00-006 step 7 NOTE) |
| P-108 | Zero bytes after the CR in an input report are not data. | `transport.Stream` (stops at CR) | none | UNVERIFIED |

## 7. Serial

| No. | Fact | Code | Source | Status |
|-----|------|------|--------|--------|
| P-120 | 2400 bit/s, 8 data bits, no parity, 1 stop bit. | `transport.NewSerial` | IS2 03-00-00-006 step 4 | UNVERIFIED for the Reference UPS |
| P-121 | DTR on and RTS off at open time. | `transport.DefaultSerialOptions` | IS2 03-00-00-006 step 4 | UNVERIFIED for the Reference UPS |
