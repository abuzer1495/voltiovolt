// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth is the PIN authentication of vltd (VLT-IS-003 TASK
// 03-00-00-005).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PIN length limits (step 1).
const (
	MinPINLength = 6
	MaxPINLength = 64
)

// Params are the Argon2id parameters.
type Params struct {
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
	SaltLen   uint32
	KeyLen    uint32
}

// DefaultParams are the parameters of step 3: time 3, memory 64 MiB,
// threads 4, salt 16 bytes, key 32 bytes. Source: RFC 9106, section 4,
// second recommended option.
var DefaultParams = Params{Time: 3, MemoryKiB: 64 * 1024, Threads: 4, SaltLen: 16, KeyLen: 32}

// Errors.
var (
	ErrPINLength = fmt.Errorf("auth: PIN must have %d to %d characters", MinPINLength, MaxPINLength)
	ErrPINChars  = errors.New("auth: PIN must contain only digits and letters (A-Z, a-z, 0-9)")
	ErrPINWeak   = errors.New("auth: PIN is too simple")
	ErrBadHash   = errors.New("auth: stored PIN hash is not valid")
)

// commonPINs is the list of 20 common PINs of step 2 (Builder list). Other
// simple PINs (same character only, simple sequences) are found by rule.
var commonPINs = [20]string{
	"123123", "121212", "112233", "123321", "696969",
	"159753", "147258", "789456", "102030", "520520",
	"password", "qwerty", "qwertyuiop", "abc123", "letmein",
	"iloveyou", "admin1", "welcome", "monkey", "dragon",
}

// ValidatePIN checks the PIN rules of steps 1 and 2.
func ValidatePIN(pin string) error {
	if len(pin) < MinPINLength || len(pin) > MaxPINLength {
		return ErrPINLength
	}
	for i := 0; i < len(pin); i++ {
		c := pin[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return ErrPINChars
		}
	}
	low := strings.ToLower(pin)
	if sameChar(low) || sequence(low) {
		return ErrPINWeak
	}
	for _, c := range commonPINs {
		if low == c {
			return ErrPINWeak
		}
	}
	return nil
}

func sameChar(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

// sequence reports a simple ascending or descending sequence of digits
// (with wrap from 9 to 0, for example "1234567890") or of letters
// ("abcdef", "fedcba").
func sequence(s string) bool {
	step := func(a, b byte) int {
		if isDigit(a) && isDigit(b) {
			d := (int(b) - int(a) + 10) % 10
			switch d {
			case 1:
				return 1
			case 9:
				return -1
			}
			return 0
		}
		if !isDigit(a) && !isDigit(b) {
			return int(b) - int(a)
		}
		return 0
	}
	d := step(s[0], s[1])
	if d != 1 && d != -1 {
		return false
	}
	for i := 2; i < len(s); i++ {
		if step(s[i-1], s[i]) != d {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

var b64 = base64.RawStdEncoding

// Hash returns the PHC string of the PIN:
// $argon2id$v=19$m=<KiB>,t=<time>,p=<threads>$<salt>$<hash>
// (salt and hash in base64 without padding).
func Hash(pin string, p Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pin), salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Time, p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// parsePHC parses a PHC string made by Hash.
func parsePHC(s string) (Params, []byte, []byte, error) {
	var p Params
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, ErrBadHash
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return p, nil, nil, ErrBadHash
	}
	var threads uint32
	if n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &threads); err != nil || n != 3 {
		return p, nil, nil, ErrBadHash
	}
	// Refuse parameters that are too large (time and memory limits).
	if p.Time < 1 || p.Time > 16 || p.MemoryKiB < 8 || p.MemoryKiB > 1<<20 || threads < 1 || threads > 16 {
		return p, nil, nil, ErrBadHash
	}
	p.Threads = uint8(threads)
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return p, nil, nil, ErrBadHash
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < 16 {
		return p, nil, nil, ErrBadHash
	}
	p.SaltLen, p.KeyLen = uint32(len(salt)), uint32(len(key))
	return p, salt, key, nil
}

// Verify compares the PIN with the PHC string in constant time.
func Verify(pin, phc string) (bool, error) {
	p, salt, want, err := parsePHC(phc)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(pin), salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
