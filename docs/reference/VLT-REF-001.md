# VOLTIOVOLT (VLT UPS MANAGEMENT)
## REFERENCE DOCUMENT
## MEGATEC SERIAL PROTOCOL AND RELATED DIALECTS – INTEROPERABILITY REFERENCE

### DOCUMENT IDENTIFICATION

- Document number: VLT-REF-001
- Revision: 00
- Date: 2026-11-10
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Type: Reference data. This is not an instruction.

### SOURCE

- Owner interoperability analysis of UPSilon 2000 v5.2 (RupsMon 2.0.278, Mega System Technologies), 2026-11-10.
- The analysis was static. It was not compared with live traffic.
- This document describes observable behavior only: commands, timing, byte positions and rules. It contains no code from the analyzed software.
- All examples in this document are synthetic. They have the correct format. They are not copied from the analyzed software.

### STATUS LABELS

| Label | Meaning |
|-------|---------|
| FIELD | Confirmed by field use on the Reference UPS computer. |
| STATIC | From the static analysis. Not verified with live traffic. |
| PUBLIC | Also in the public Megatec protocol document. |

### FIELD EVIDENCE

1. On the Reference UPS computer, the UPS is connected with a USB cable. Windows shows it as a virtual COM port (COM3). FIELD.
2. UPSilon 2000 on that computer uses the communication type "MegaTec" on COM3. FIELD.
3. UPSilon 2000 recorded data every 30 seconds from 2026-10-18 to 2026-11-10 without a gap. The data shows 230 V and 50 Hz. FIELD.
4. Result: the serial Megatec path with Q1, F and I works with the Reference UPS. FIELD.
5. UPSilon 2000 does not use QS, QGS, QBV, QMOD, QWS, QFLAG, QRI, QID, QVFW or QMD. This does not prove that the Reference UPS does not support them. Only a Capture can show it.
6. The USB-to-serial bridge chip and its VID and PID are not known.

### USE RULE

1. Use these facts with SEC-060.
2. If a fact has the label STATIC only, write UNVERIFIED in the code comment and in docs/protocol.md.
3. Do not ask for more detail from private sources. Ask the Instruction Author.

---

## 1. Dialects

| Dialect (UPSilon name) | Physical path | Section | VLT release |
|------------------------|---------------|---------|-------------|
| MegaTec | Serial port (RS-232 or USB virtual COM port) | 2 to 7 | MVP |
| Mega(USB) | USB HID, commands as USB string descriptor requests | 8 | v2.0 |
| SEC (2400 or 9600 bit/s) | Serial port | 9 | v2.0 |

UPSilon 2000 has no automatic dialect detection. The user selects the dialect and the port. STATIC.

---

## 2. Megatec serial link

| Parameter | Value | Status |
|-----------|-------|--------|
| Bit rate | 2400 bit/s | STATIC, PUBLIC |
| Data bits, parity, stop bits | 8, none, 1 | STATIC, PUBLIC |
| Flow control, DTR, RTS | UPSilon 2000 does not set them. The port keeps the values of the driver. | STATIC |
| Port | Selected by the user from the serial ports of the OS | STATIC |

NOTE: Some Megatec cables need a specific DTR or RTS level. Because UPSilon 2000 does not set these lines and works on the Reference UPS, the driver default values work for the Reference UPS.

---

## 3. Megatec framing and timing

1. A command is ASCII text, then CR (0x0D). There is no prefix, no length field and no checksum. The command is written in one write operation. STATIC, PUBLIC.
2. A response is ASCII text, then CR. The CR marks the end of the response. STATIC, PUBLIC.
3. UPSilon 2000 waits a maximum of 500 ms for the CR after a command. If no CR comes, the query has no response, and UPSilon 2000 clears the transmit buffer. STATIC.
4. UPSilon 2000 sends a maximum of one command every 2 seconds. STATIC.
5. The receive buffer of UPSilon 2000 has 200 bytes. STATIC.
6. Connection state:
    1. "Connected": the first response that ends with CR after the port opens. STATIC.
    2. Each response that ends with CR (valid or not valid) sets the no-response counter to zero. STATIC.
    3. "Disconnected": when the number of queries without a response in sequence reaches the limit below. STATIC.

| Last valid status | Limit |
|-------------------|-------|
| Normal | 4 |
| Bit b7 = 1 (utility fail) | 5 |
| Bit b1 = 1 (shutdown active) | 7 |

---

## 4. Megatec command sequence of UPSilon 2000

STATIC. This is reference behavior. VLT does not have to copy it.

1. After the port opens: F, then Q1.
2. Normal polling: a cycle of 11 steps, one step every 2 seconds.

| Step | 0 | 1 | 2 | 3 | 4 to 10 |
|------|---|---|---|---|---------|
| Command | Q1 | F | Q1 | I | Q1 |

3. Result: Q1 approximately every 2 seconds. F and I every 22 seconds.
4. User commands replace a polling step. Priority: shutdown, test cancel, test, beeper.
5. Shutdown command: UPSilon 2000 sends it up to 3 times, with Q1 between the sends, 2 seconds apart. If a Q1 response after a send shows b1 = 1, UPSilon 2000 stops the repeats.
6. Test command (T, TL, Tnn): the same pattern. UPSilon 2000 stops the repeats when Q1 shows b2 = 1.
7. Test cancel (CT) and beeper toggle (Q): one send.
8. UPSilon 2000 never sends C (cancel shutdown).

---

## 5. Megatec command list

| Command bytes | Function | Response | Status |
|---------------|----------|----------|--------|
| Q1 CR | Status inquiry | 6.1 | STATIC, PUBLIC |
| F CR | Rating information | 6.2 | STATIC, PUBLIC |
| I CR | Company, model, version | 6.3 | STATIC, PUBLIC |
| T CR | Battery test, 10 seconds | Not read | STATIC, PUBLIC |
| TL CR | Battery test until battery low | Not read | STATIC, PUBLIC |
| Tnn CR | Battery test for nn minutes, nn = 01 to 99, two digits with leading zero | Not read | STATIC, PUBLIC |
| CT CR | Cancel test | Not read | STATIC, PUBLIC |
| Q CR | Beeper toggle (on to off, off to on) | Not read | STATIC, PUBLIC |
| S... CR | Shutdown (5.1) | Not read | STATIC, PUBLIC |
| S...R.... CR | Shutdown and restore (5.1) | Not read | STATIC, PUBLIC |
| C CR | Cancel shutdown | Not read | PUBLIC only. UNVERIFIED. UPSilon 2000 never sends it. |

"Not read": UPSilon 2000 does not parse a response to these commands. A response with CR sets the no-response counter to zero.

WARNING: Q, T, TL, Tnn, CT, S, S...R and C are CONTROL commands. "Q" is also the first byte of "Q1". A partial write of "Q1 CR" can send "Q" (SEC-038).

### 5.1 Shutdown encoding

Inputs: D = delay before shutdown, in seconds. R = restore time, in minutes. STATIC.

| Condition | Bytes on the line | Example |
|-----------|-------------------|---------|
| D less than 60, R = 0 | "S." + one digit + CR. The digit is the first decimal of D/60, rounded to one decimal. | D = 30: "S.5" CR. D = 12: "S.2" CR. |
| D less than 60, R more than 0 | "S." + one digit + "R" + R with 4 digits and leading zeros + CR | D = 30, R = 1: "S.5R0001" CR |
| D 60 or more, R = 0 | "S" + whole minutes of D with 2 digits and leading zero + CR | D = 300: "S05" CR |
| D 60 or more, R more than 0 | "S" + whole minutes with 2 digits + "R" + R with 4 digits + CR | D = 120, R = 10: "S02R0010" CR |

1. UPSilon 2000 range: D from 12 to 5940 seconds, R from 0 to 9999 minutes. It does not send values outside the range. STATIC.
2. Defect in UPSilon 2000: D = 57, 58 or 59 rounds to 1.0, and the bytes are "S.0" CR. VLT MUST NOT send "S.0". STATIC.
3. The public Megatec document gives the delay as .2 to .9 (minutes) or 01 to 10 (minutes). PUBLIC.
4. The behavior of restore when mains power is not present is not known. UNVERIFIED.

---

## 6. Megatec responses

UPSilon 2000 selects the response type by the first byte and the position of the CR. STATIC.

| First byte | CR position (0-based) | Response type |
|------------|------------------------|---------------|
| "(" | 46 | Q1 |
| "#" | 21 | F |
| "#" | 38 | I |

A response that starts with "(" but fails the position checks is ignored. The values do not change. STATIC.

### 6.1 Q1 response (47 bytes)

Checks: byte 0 = "("; bytes 6, 12, 18, 22, 27, 32 and 37 = space; byte 46 = CR. STATIC, PUBLIC.

| Offset | Length | Field | Unit | Resolution kept by UPSilon 2000 |
|--------|--------|-------|------|----------------------------------|
| 1 | 5 | Input voltage | V | 0.1 |
| 7 | 5 | Input fault voltage | V | 0.1 |
| 13 | 5 | Output voltage | V | 0.1 |
| 19 | 3 | Load | % | 1 |
| 23 | 4 | Frequency | Hz | 0.1 |
| 28 | 4 | Battery voltage | V (per cell on an on-line UPS; total on a standby UPS) | 0.01 |
| 33 | 4 | Temperature | °C | 0.1 |
| 38 | 8 | Status bits b7 to b0, characters "0" or "1" | none | |

NOTE: A numeric field can have leading spaces. Example: the load field "  7". STATIC.

Status bits. Character 38 is b7. STATIC, PUBLIC.

| Bit | Character position | Meaning when 1 |
|-----|--------------------|----------------|
| b7 | 38 | Utility fail (UPS on battery) |
| b6 | 39 | Battery low |
| b5 | 40 | AVR or bypass active. If b3 = 1: boost when input voltage is less than output voltage, else buck. If b3 = 0: bypass. |
| b4 | 41 | UPS failed |
| b3 | 42 | UPS type standby (line-interactive). 0 = on-line. |
| b2 | 43 | Test in progress |
| b1 | 44 | Shutdown active |
| b0 | 45 | Beeper on |

UPSilon 2000 makes an event at each change of a bit (0 to 1 and 1 to 0). STATIC.

Synthetic examples (CR not shown):

| Case | Response |
|------|----------|
| On-line UPS, mains, beeper on | `(229.8 229.8 219.9 034 50.1 2.25 24.0 00000001` |
| On-line UPS, on battery | `(000.0 228.0 220.0 035 50.0 2.10 24.5 10000001` |
| Standby UPS, mains, load with leading spaces | `(230.1 230.1 230.1   7 50.0 27.0 25.0 00001000` |

### 6.2 F response (22 bytes)

Checks: byte 0 = "#"; bytes 6, 10 and 16 = space; byte 21 = CR. STATIC, PUBLIC.

| Offset | Length | Field | Unit |
|--------|--------|-------|------|
| 1 | 5 | Rated voltage | V |
| 7 | 3 | Rated current | A |
| 11 | 5 | Rated battery voltage | V |
| 17 | 4 | Rated frequency | Hz |

NOTE: The number format in a field can change. Examples: rated current "004" or "4.3"; rated battery voltage "024.0" or "24.00". STATIC.

Synthetic examples (CR not shown):

| Response |
|----------|
| `#220.0 004 024.0 50.0` |
| `#230.0 4.3 24.00 50.0` |

### 6.3 I response (39 bytes)

Checks: byte 0 = "#"; bytes 16 and 27 = space; byte 38 = CR. STATIC, PUBLIC.

| Offset | Length | Field |
|--------|--------|-------|
| 1 | 15 | Company name, filled with spaces on the right |
| 17 | 10 | UPS model |
| 28 | 10 | Version |

Synthetic example (CR not shown; the spaces are part of the response):

    #ACME POWER      ONL-1K     VER01.00  

If the I or the F query has no response, UPSilon 2000 uses stored model and rating data. The line behavior does not change. STATIC.

---

## 7. Derived value: battery capacity

The Q1 response gives no battery capacity. UPSilon 2000 calculates it. STATIC. Vb = battery voltage from Q1. Vn = rated battery voltage from F.

1. If Vb = 0, capacity = 100 %.
2. On-line UPS (b3 = 0): Vb is the voltage per cell. Capacity = (Vb − 1.667) × 166.667. Result: 1.667 V = 0 %, 2.267 V = 100 %.
3. Standby UPS (b3 = 1): if Vn = 0, capacity = 100 %. Else k = Vn / 24 and capacity = 100 × (Vb − 20k) / (7.2k). Result per 12 V block: 10.0 V = 0 %, 13.6 V = 100 %.
4. The result is changed to an integer. A value more than 90 becomes 100. A value less than 0 becomes 0.

---

## 8. Mega(USB) dialect (USB HID, string descriptors)

STATIC. Not used on the Reference UPS. VLT release v2.0.

1. The UPS is a USB HID device. The commands do not use HID reports. Each command is a request for a USB string descriptor with a specific index. The UPS returns the response as the string content.
2. On Windows, UPSilon 2000 uses HidD_GetIndexedString(index, length). On the bus, this is a GET_DESCRIPTOR (String) control transfer on endpoint 0. The language ID (wIndex) is not known. It is not known if the length goes to wLength without change. UNVERIFIED.
3. The response is UTF-16LE. Its content is the same as the ASCII response in section 6, without the CR.
4. Device recognition: UPSilon 2000 does not check VID or PID. It requests index 3 from each HID device. If the first character of the result is "(" or "U", the device is accepted.
5. A response that starts with "UPS" is a "no response" message. After 4 of them in sequence, the connection is lost.
6. If the product string is "MEC0003", the restore encoding is different (step 8 table).

| Megatec command | String index | Requested length (bytes) |
|-----------------|--------------|--------------------------|
| Q1 | 0x03 | 100 |
| T (10 s) | 0x04 | 100 |
| TL | 0x05 | 100 |
| Tnn | 0x06 | 0x62 + BCD(nn). Example: nn = 5 gives 0x67, nn = 12 gives 0x74. |
| Q (beeper) | 0x07 | 100 |
| CT | 0x0B | 100 |
| I | 0x0C | 100 |
| F | 0x0D | 100 |
| S (no restore) | (code shifted left 4) OR 8 | 100 |
| S with restore | (code shifted left 4) OR x | See step 8 |

7. Shutdown delay code (D in seconds):

| D | 30 or less | 36 | 42 | 48 | 54 | 60 or more |
|---|------------|----|----|----|----|------------|
| Code | 1 | 2 | 3 | 4 | 5 | min(D, 600) / 60 + 5, which gives 6 to 15 |

Values from 31 to 59 that are not in the table give code 5.

8. Restore (R in minutes):
    1. Product string "MEC0003": b = 0 if R is less than 4000, 1 if R is less than 8000, else 2. x = b. Length = R − 4000 × b + 2.
    2. Other devices: if the last digit of R is more than 7, it becomes 7. Maximum R = 1997. x = 9. Length = R as a 4-digit BCD value. If this value is more than 0x999, length = (value AND 0xFF0) + 10.

---

## 9. SEC dialect (2400 or 9600 bit/s)

STATIC, low confidence. Not used on the Reference UPS. VLT release v2.0.

1. Link: 2400 or 9600 bit/s (the user selects), 8 data bits, no parity, 1 stop bit.
2. Command format: "^" + type letter ("P" = query, "S" = set or command) + length with 3 digits + body. The length is the number of characters in the body. There is no CR.
3. Response format: "^D" + length with 3 digits + fields separated by commas. A valid response has 8 bytes or more and contains "^D". Field n is the integer after comma n.
4. Timing: after a command, wait 500 ms, then read. If fewer than 3 bytes come, send the command again, for a maximum of 3 seconds. Read every 500 ms. 4 invalid reads in sequence = connection lost.

| Query | Fields used by UPSilon 2000 |
|-------|------------------------------|
| ^P003ST2 | field 2 × 0.1 = input frequency (Hz); field 3 × 0.1 = input voltage (V) |
| ^P003ST3 | field 3 × 0.1 = output voltage (V); field 4 × 0.1 = load (%) |
| ^P003ST1 | field 1 = battery state; field 5 / 10 = capacity (%); field 6 = battery voltage (× 0.1 V if less than 1000, else × 0.01 V); field 8 = temperature (°C) |
| ^P003ST5, ^P003STR | status data (details not known) |
| ^P003NOM | rated values |
| ^P003MAN, ^P003MOD, ^P003VER | company, model, version |

| Command | Function |
|---------|----------|
| ^S004TST2 | Test, 10 seconds |
| ^S004TST3 | Test until battery low |
| ^S004TST1 | Test for a time |
| ^S005TST-1 | Cancel test |
| ^S011NOM,,,,,,,2 | Alarm or beeper setting (equivalent of Megatec Q) |
| "^S00" + length + "PSD" + seconds | Shutdown delay. Example: 60 s gives ^S005PSD60. |
| "^S00" + length + "RWD" + seconds | Restore or wake setting. UPSilon 2000 also writes the shutdown delay here. |
| ^S004SDA1, ^S004ATR1 | Sent before a shutdown (function not known) |

5. Shutdown sequence: ^S004ATR1, wait 500 ms, ^S004SDA1, wait 500 ms, the RWD command if R is more than 0 (wait 10 ms), then the PSD command. Repeat every 2 seconds, up to 3 times. Stop when the UPS status shows shutdown active.
6. Test and alarm commands: repeat every 500 ms until the first two bytes of the response are "^1", for a maximum of 3 seconds.

---

## 10. Open items

1. Live traffic on the Reference UPS (Owner Capture, test T-01): bit rate, response time, exact Q1 format, DTR and RTS need.
2. The scale of the F battery voltage compared with the Q1 battery voltage on the Reference UPS.
3. Support of QS, QGS, QBV, QMOD, QWS, QFLAG, QRI, QID, QVFW and QMD on the Reference UPS.
4. Support of C (cancel shutdown) on the Reference UPS.
5. Restore behavior of S...R when mains power is not present.
6. Mega(USB): language ID and wLength.
7. SEC: full field list.

END OF DOCUMENT VLT-REF-001
