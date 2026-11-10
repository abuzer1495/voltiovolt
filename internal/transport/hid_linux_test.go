// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListHIDSysfs(t *testing.T) {
	root := t.TempDir()
	mk := func(name, uevent string, desc []byte) {
		dir := filepath.Join(root, name, "device")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dir, "uevent"), []byte(uevent), 0o644)
		if desc != nil {
			_ = os.WriteFile(filepath.Join(dir, "report_descriptor"), desc, 0o644)
		}
	}
	desc := []byte{0x75, 0x08, 0x95, 0x08, 0x81, 0x02, 0x91, 0x02}
	mk("hidraw0", "HID_ID=0003:0000046D:0000C077\nHID_NAME=Mouse\n", nil)
	mk("hidraw1", "HID_ID=0003:00000665:00005161\nHID_NAME=FAKE UPS\nHID_UNIQ=FAKE-SERIAL\n", desc)
	devs, err := listHIDIn(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 {
		t.Fatalf("%d devices", len(devs))
	}
	d := devs[0]
	if d.Path != "/dev/hidraw1" || d.Product != "FAKE UPS" || d.Serial != "FAKE-SERIAL" {
		t.Errorf("%+v", d)
	}
	if d.Lengths.Input != 8 || d.Lengths.Output != 8 {
		t.Errorf("lengths %+v", d.Lengths)
	}
	if devs, err := listHIDIn(filepath.Join(root, "missing")); err != nil || devs != nil {
		t.Errorf("%v %v", devs, err)
	}
}
