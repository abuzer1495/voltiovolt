// SPDX-License-Identifier: AGPL-3.0-or-later

package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func frame(b []byte) []byte {
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out, uint32(len(b)))
	copy(out[4:], b)
	return out
}

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	id := int64(7)
	if err := WriteFrame(&buf, Request{V: 1, ID: &id, Method: MethodHello}); err != nil {
		t.Fatal(err)
	}
	b, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeRequest(b)
	if err != nil || *r.ID != 7 || r.Method != MethodHello || string(r.Params) != "{}" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestFrameLimits(t *testing.T) {
	big := bytes.Repeat([]byte("a"), MaxFrame+1)
	if _, err := ReadFrame(bytes.NewReader(frame(big))); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	if _, err := ReadFrame(bytes.NewReader([]byte{0, 0, 0, 0})); !errors.Is(err, ErrEmptyFrame) {
		t.Fatalf("empty: %v", err)
	}
	if err := WriteFrame(&bytes.Buffer{}, map[string]string{"a": strings.Repeat("x", MaxFrame)}); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("write too large: %v", err)
	}
	// Exactly 1 MiB is permitted.
	ok := append([]byte(`"`), append(bytes.Repeat([]byte("a"), MaxFrame-2), '"')...)
	if b, err := ReadFrame(bytes.NewReader(frame(ok))); err != nil || len(b) != MaxFrame {
		t.Fatalf("1 MiB frame: %v", err)
	}
}

func TestDecodeRequestStrict(t *testing.T) {
	bad := []string{
		`{"v":1,"id":1,"method":"system.hello","extra":1}`,
		`{"v":2,"id":1,"method":"system.hello"}`,
		`{"v":1,"method":"system.hello"}`,
		`{"v":1,"id":1}`,
		`{"v":1,"id":1,"method":"x","params":[1]}`,
		`{"v":1,"id":1,"method":"x","params":"a"}`,
		`{"v":1,"id":1,"method":"x"} {}`,
		`{"v":1,"id":"1","method":"x"}`,
		`[]`, `null`, ``, `{`,
	}
	for _, b := range bad {
		if _, err := DecodeRequest([]byte(b)); err == nil {
			t.Errorf("accepted %s", b)
		}
	}
	var p PINParams
	if err := Strict([]byte(`{"pin":"x","other":1}`), &p); err == nil {
		t.Error("unknown param field accepted")
	}
}

// FuzzDecodeFrame is the fuzz target for the frame decoder (step 8).
func FuzzDecodeFrame(f *testing.F) {
	f.Add(frame([]byte(`{"v":1,"id":1,"method":"system.hello","params":{}}`)))
	f.Add(frame([]byte(`{"v":1,"id":1,"method":"auth.login","params":{"pin":"x"}}`)))
	f.Add([]byte{0, 0x10, 0, 1})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, in []byte) {
		b, err := ReadFrame(bytes.NewReader(in))
		if err != nil {
			return
		}
		if len(b) > MaxFrame {
			t.Fatal("frame larger than the limit")
		}
		r, err := DecodeRequest(b)
		if err != nil {
			return
		}
		if r.V != Version || r.ID == nil || r.Method == "" || len(r.Params) == 0 || r.Params[0] != '{' {
			t.Fatalf("accepted a request that is not valid: %+v", r)
		}
	})
}
