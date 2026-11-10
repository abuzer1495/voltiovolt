// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"errors"
	"fmt"
)

// ErrNotRead is returned when a string is not a READ command in the registry.
var ErrNotRead = errors.New("dialect: not a READ command in the registry")

// EncodeRead makes the request bytes for one READ command: the ASCII command,
// then CR. It rejects all strings that are not READ commands in the registry.
// There is no other encoder in VLT.
func EncodeRead(name string) ([]byte, error) {
	e, ok := LookupRead(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotRead, name)
	}
	out := make([]byte, 0, len(e.Name)+1)
	out = append(out, e.Name...)
	out = append(out, '\r')
	return out, nil
}
