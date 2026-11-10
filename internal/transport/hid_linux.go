// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package transport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const sysHidraw = "/sys/class/hidraw"

// ListHID lists the hidraw devices with VID 0665 and PID 5161. It finds the
// devices through sysfs. It opens no device and sends no command.
func ListHID() ([]HIDDevice, error) {
	return listHIDIn(sysHidraw)
}

func listHIDIn(root string) ([]HIDDevice, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []HIDDevice
	for _, e := range entries {
		dir := filepath.Join(root, e.Name(), "device")
		d, ok := readUevent(filepath.Join(dir, "uevent"))
		if !ok || d.VID != VendorID || d.PID != ProductID {
			continue
		}
		d.Path = "/dev/" + e.Name()
		if desc, err := os.ReadFile(filepath.Join(dir, "report_descriptor")); err == nil {
			d.ReportDescriptor = desc
			if l, err := ParseReportDescriptor(desc); err == nil {
				d.Lengths = l
			}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// readUevent reads HID_ID, HID_NAME and HID_UNIQ from a uevent file.
// HID_ID has the format "BUS:VVVVVVVV:PPPPPPPP" in hexadecimal.
func readUevent(path string) (HIDDevice, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return HIDDevice{}, false
	}
	var d HIDDevice
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "HID_ID":
			p := strings.Split(v, ":")
			if len(p) != 3 {
				return HIDDevice{}, false
			}
			vid, err1 := strconv.ParseUint(p[1], 16, 32)
			pid, err2 := strconv.ParseUint(p[2], 16, 32)
			if err1 != nil || err2 != nil || vid > 0xffff || pid > 0xffff {
				return HIDDevice{}, false
			}
			d.VID, d.PID = uint16(vid), uint16(pid)
			found = true
		case "HID_NAME":
			d.Product = v
		case "HID_UNIQ":
			d.Serial = v
		}
	}
	return d, found
}

type hidraw struct {
	fd  int
	ids bool
	buf []byte
}

func openHID(path string) (ReportDevice, HIDDevice, error) {
	name := filepath.Base(path)
	if !strings.HasPrefix(name, "hidraw") || filepath.Dir(path) != "/dev" {
		return nil, HIDDevice{}, fmt.Errorf("transport: %q is not a hidraw device", path)
	}
	devs, err := ListHID()
	if err != nil {
		return nil, HIDDevice{}, err
	}
	var info *HIDDevice
	for i := range devs {
		if devs[i].Path == path {
			info = &devs[i]
		}
	}
	if info == nil {
		return nil, HIDDevice{}, fmt.Errorf("transport: %s is not VID 0665 PID 5161", path)
	}
	if info.ReportDescriptor == nil {
		return nil, HIDDevice{}, errors.New("transport: report descriptor not readable")
	}
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, HIDDevice{}, fmt.Errorf("transport: open %s: %w", path, err)
	}
	return &hidraw{fd: fd, ids: info.Lengths.HasReportIDs, buf: make([]byte, info.Lengths.Input+1)}, *info, nil
}

func (h *hidraw) WriteReport(r []byte) (int, error) {
	if h.fd < 0 {
		return 0, ErrClosed
	}
	// hidraw: the first byte is the report number, 0 for no report IDs.
	return unix.Write(h.fd, r)
}

func (h *hidraw) ReadReport(p []byte, timeout time.Duration) (int, error) {
	if h.fd < 0 {
		return 0, ErrClosed
	}
	ms := int(timeout.Milliseconds())
	if ms < 1 {
		ms = 1
	}
	fds := []unix.PollFd{{Fd: int32(h.fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, ms)
	if errors.Is(err, unix.EINTR) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		return 0, errors.New("transport: hidraw device error or disconnect")
	}
	m, err := unix.Read(h.fd, h.buf)
	if errors.Is(err, unix.EAGAIN) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	data := h.buf[:m]
	if h.ids && m > 0 {
		data = data[1:] // the first byte is the report number
	}
	return copy(p, data), nil
}

func (h *hidraw) Close() error {
	if h.fd < 0 {
		return nil
	}
	err := unix.Close(h.fd)
	h.fd = -1
	return err
}
