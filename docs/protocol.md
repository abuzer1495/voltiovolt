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
| R1 | VLT-REF-001 Revision 00, Owner interoperability reference (OD-24): `docs/reference/VLT-REF-001.md` |

VLT-REF-001 status labels (section 9 and later):
- **FIELD**: confirmed by field use on the Reference UPS computer.
- **STATIC**: from the static analysis. Not verified with live traffic. A fact with the label STATIC only is also UNVERIFIED (VLT-REF-001 USE RULE 2).
- **PUBLIC**: also in the public Megatec protocol document (M).

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
| P-029 | The Dialect assignment of each READ command in the registry (megatec, voltronic-qs, voltronic; names of VLT-IS-001 Rev 05, 00-10-00). | registry `Dialects` | Builder inference from A and M; megatec = Q1, F, I from R1 section 1 and 5 | UNVERIFIED (megatec: FIELD, R1) |

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

## 8. Status bits and Events (VLT-IS-004 TASK 03-00-00-005)

Source: [M] Megatec protocol document (<https://networkupstools.org/protocols/megatec.html>), Q1 status byte `b7b6b5b4b3b2b1b0`. The QS response uses the same byte (VLT-IS-002 03-50-01). Bit 7 is the first character. UNVERIFIED for the Reference UPS.

| Bit | Condition [M] | Event | Code |
|-----|---------------|-------|------|
| 7 = 1 | Utility fail | ON_BATTERY (WARNING). Back to 0: ONLINE (INFO). | `event.Engine.Observe` |
| 6 = 1 | Battery low | LOW_BATTERY (CRITICAL) | `event.Engine.Observe` |
| 5 = 1 and bit 3 = 0 | Bypass active (on-line type) | BYPASS_ACTIVE (WARNING) | `event.Engine.Observe` |
| 5 = 1 and bit 3 = 1 | Boost or buck active (standby type) | AVR_ACTIVE (INFO) | `event.Engine.Observe` |
| 4 = 1 | UPS failed | UPS_FAULT (CRITICAL) | `event.Engine.Observe` |
| 2 = 1 | Test in progress | TEST_STARTED (INFO) | `event.Engine.Observe` |
| 1 = 1 | Shutdown active | UPS_SHUTDOWN_ACTIVE (CRITICAL) | `event.Engine.Observe` |

1. Bit 3 alone (UPS type) and bit 0 (beeper on) make no Event.
2. QMOD and the QGS status bits are UNVERIFIED. vltd records them as data only and makes no Event from them. A change of the QWS value (2 polls in sequence) makes UPS_WARNING_BITS_CHANGED (INFO) with the old and the new value. The first QWS value makes no Event.
3. If one cycle has QS and Q1, the QS bits are used.
4. NOTE: with the Voltronic Dialect, QS is in the slow group (30 s). Thus ON_BATTERY and LOW_BATTERY can come up to 30 seconds after the change.

## 9. Megatec Dialect (VLT-IS-006 TASK 03-00-00-003, source R1)

The megatec Dialect is the path of the Reference UPS (OD-26). Each row has the label of VLT-REF-001. The "Status" column uses the rule above: STATIC only gives UNVERIFIED.

| No. | Fact | Code | R1 section | Label | Status |
|-----|------|------|------------|-------|--------|
| P-140 | The Reference UPS is connected with a USB cable. The OS shows a virtual COM port (COM3 on the Reference UPS computer). | `transport.NewSerial` | Field evidence 1 | FIELD | VERIFIED |
| P-141 | The serial Megatec path with Q1, F and I works with the Reference UPS. | Dialect `megatec` | Field evidence 4 | FIELD | VERIFIED |
| P-142 | The USB-to-serial bridge chip and its VID and PID are not known. | none | Field evidence 6 | FIELD | UNVERIFIED (not known) |
| P-143 | HID VID 0665 PID 5161 is not the path of the Reference UPS. It stays supported (OD-26). | `transport.VendorID`, `ProductID` | Field evidence 2, 4 | FIELD | VERIFIED (OD-26) |
| P-144 | 2400 bit/s, 8 data bits, no parity, 1 stop bit. | `transport.NewSerial` | 2 | STATIC, PUBLIC | VERIFIED (M) |
| P-145 | UPSilon 2000 does not set flow control, DTR or RTS. The port keeps the values of the driver. | `transport.SerialOptions` (VLT-IS-006 TASK 03-00-00-004) | 2 | STATIC | UNVERIFIED |
| P-146 | A command is ASCII text, then CR, written in one write operation. No prefix, no length, no checksum. A response is ASCII text, then CR. | `dialect.EncodeRead`, `transport.Stream` | 3.1, 3.2 | STATIC, PUBLIC | VERIFIED (M) |
| P-147 | UPSilon 2000 waits a maximum of 500 ms for the CR and sends a maximum of one command every 2 seconds. VLT uses its own values: response timeout 1500 ms, minimum gap 500 ms, fast group every 2 s. | `device.ResponseTimeout`, `device.MinGap`, `poller.FastPeriod` | 3.3, 3.4 | STATIC | UNVERIFIED (reference behavior only) |
| P-148 | Megatec command list: Q1, F, I (READ); T, TL, Tnn (nn = 01 to 99), CT, Q, S..., S...R..., C (CONTROL). | registry | 5 | STATIC, PUBLIC; C: PUBLIC only | VERIFIED (M); C UNVERIFIED |
| P-149 | The receiver selects the response type by the first byte and the CR position: "(" and CR at 46 = Q1; "#" and CR at 21 = F; "#" and CR at 38 = I. | `dialect.ParseQ1`, `ParseF`, `ParseI` | 6 | STATIC | UNVERIFIED |
| P-150 | Q1: 47 bytes with the CR. Byte 0 "("; bytes 6, 12, 18, 22, 27, 32, 37 are spaces; byte 46 is CR. | `dialect.ParseQ1`, `Q1Len` | 6.1 | STATIC, PUBLIC | VERIFIED (M) |
| P-151 | Q1 fields (offset, length): input V (1, 5), input fault V (7, 5), output V (13, 5), load % (19, 3), frequency Hz (23, 4), battery V (28, 4), temperature °C (33, 4), status bits (38, 8). | `dialect.q1Spans` | 6.1 | STATIC, PUBLIC | VERIFIED (M) |
| P-152 | Q1 battery voltage: per cell on an on-line UPS, total on a standby UPS. | `measure.ChargeEstimate` | 6.1 | STATIC, PUBLIC | VERIFIED (M) |
| P-153 | A numeric field can have leading spaces (example load "  7"). | `dialect.fixedQuantity` | 6.1 NOTE | STATIC | UNVERIFIED |
| P-154 | A numeric field with only spaces has no value. | `dialect.fixedQuantity` | VLT-IS-006 TASK 03-00-00-003 step 2.4 (VLT rule) | none | VERIFIED (rule) |
| P-155 | Status bits: character 38 is b7. b7 utility fail, b6 battery low, b5 AVR or bypass, b4 UPS failed, b3 standby type (0 = on-line), b2 test in progress, b1 shutdown active, b0 beeper on. | `dialect.Q1Flags` | 6.1 | STATIC, PUBLIC | VERIFIED (M) |
| P-156 | b5 = 1 and b3 = 1: boost when the input voltage is less than the output voltage, else buck. b5 = 1 and b3 = 0: bypass. | `event.avrMode` (AVR_ACTIVE data "mode") | 6.1 | STATIC, PUBLIC | VERIFIED (M); boost or buck from the voltages: STATIC, UNVERIFIED |
| P-157 | F: 22 bytes with the CR. Byte 0 "#"; bytes 6, 10, 16 are spaces; byte 21 is CR. Fields: rated V (1, 5), rated current A (7, 3), rated battery V (11, 5), rated frequency Hz (17, 4). | `dialect.ParseF`, `FLen` | 6.2 | STATIC, PUBLIC | VERIFIED (M) |
| P-158 | F: the number format in a field can change: "004" or "4.3", "024.0" or "24.00". | `dialect.isNumber` | 6.2 NOTE | STATIC | UNVERIFIED |
| P-159 | I: 39 bytes with the CR. Byte 0 "#"; bytes 16 and 27 are spaces; byte 38 is CR. Fields: company (1, 15), model (17, 10), version (28, 10), filled with spaces on the right. | `dialect.ParseI`, `ILen` | 6.3 | STATIC, PUBLIC | VERIFIED (M) |
| P-160 | The Q1 response has no battery capacity. VLT calculates battery.charge_estimate: on-line linear 1.667 V to 2.267 V per cell; standby linear 10.0 V to 13.6 V per 12 V block, blocks = F rated battery V / 12. VLT does not round a value above 90 to 100 (VLT-IS-006, different from UPSilon 2000). | `measure.ChargeEstimate` | 7 | STATIC | UNVERIFIED |
| P-161 | The scale of the F battery voltage compared with the Q1 battery voltage on the Reference UPS is not known. | `measure.ChargeEstimate` | 10.2 | STATIC | UNVERIFIED (Owner test T-01) |
| P-162 | Synthetic test vectors of 6.1, 6.2 and 6.3 (not captures). | `internal/dialect/megatec_test.go` | 6 | synthetic | test data |

Probe List (VLT-IS-006 TASK 03-00-00-003 step 4): Q1, F, I, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, QID, QVFW, QMD. Dialect selection: voltronic if QGS answers, voltronic-qs if QS answers, megatec if Q1 answers. Poll groups of megatec: fast Q1 (2 s), slow none, once F and I (at start and every 6 h).
