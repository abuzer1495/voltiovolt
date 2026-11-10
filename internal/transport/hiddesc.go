// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import "errors"

// Report types in a HID report descriptor.
const (
	reportInput   = 0
	reportOutput  = 1
	reportFeature = 2
)

// ParseReportDescriptor reads the main items of a HID report descriptor and
// returns the largest input, output and feature report lengths, in bytes,
// without the report ID byte. The rules come from the USB HID specification
// 1.11, section 6.2.2 (short items, global items, main items).
func ParseReportDescriptor(d []byte) (ReportLengths, error) {
	type globals struct{ size, count, id uint32 }
	var g globals
	var stack []globals
	bits := map[uint32]*[3]uint32{}
	usesIDs := false
	for i := 0; i < len(d); {
		prefix := d[i]
		if prefix == 0xFE { // long item
			if i+2 >= len(d) {
				return ReportLengths{}, errors.New("transport: short long item")
			}
			i += 3 + int(d[i+1])
			continue
		}
		size := int(prefix & 0x03)
		if size == 3 {
			size = 4
		}
		if i+1+size > len(d) {
			return ReportLengths{}, errors.New("transport: report descriptor item is cut")
		}
		var v uint32
		for k := 0; k < size; k++ {
			v |= uint32(d[i+1+k]) << (8 * k)
		}
		typ := (prefix >> 2) & 0x03
		tag := prefix >> 4
		i += 1 + size
		switch typ {
		case 0: // main
			kind := -1
			switch tag {
			case 0x8:
				kind = reportInput
			case 0x9:
				kind = reportOutput
			case 0xB:
				kind = reportFeature
			}
			if kind >= 0 {
				b := bits[g.id]
				if b == nil {
					b = &[3]uint32{}
					bits[g.id] = b
				}
				b[kind] += g.size * g.count
			}
		case 1: // global
			switch tag {
			case 0x7:
				g.size = v
			case 0x8:
				g.id = v
				usesIDs = true
			case 0x9:
				g.count = v
			case 0xA:
				stack = append(stack, g)
			case 0xB:
				if len(stack) == 0 {
					return ReportLengths{}, errors.New("transport: pop without push")
				}
				g = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		}
	}
	var l ReportLengths
	l.HasReportIDs = usesIDs
	for id, b := range bits {
		in, out, feat := int((b[0]+7)/8), int((b[1]+7)/8), int((b[2]+7)/8)
		l.Input = max(l.Input, in)
		if out > l.Output {
			l.Output = out
			l.OutputID = byte(id)
		}
		l.Feature = max(l.Feature, feat)
	}
	return l, nil
}
