// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// SEC-096: the front end never receives a Secret value. A Secret goes from
// the input field to SetSecret, then to vltd with secret.set. SetSecret
// clears its buffer after the call. No method of API returns a Secret
// value: SecretStatus gives only set, hint and the time (TestSEC096).

// SetSecret calls secret.set (elevated). value is the UTF-8 text of the
// input field (base64 in the JSON of the binding). The buffer is cleared
// after the call, also after an error.
func (a *API) SetSecret(id string, value []byte) error {
	defer clear(value)
	c, cancel := ctx()
	defer cancel()
	p := secretParams{ID: id, Value: value}
	defer p.wipe()
	err := a.m.call(c, func(cl *client.Client) error { return cl.Call(c, wire.MethodSecretSet, &p, nil) })
	return wrap(err)
}

// secretParams is wire.SecretSetParams with a []byte value, so that the
// Go side keeps no string copy of the Secret that it cannot clear.
type secretParams struct {
	ID    string
	Value []byte
	buf   []byte
}

// MarshalJSON writes {"id":...,"value":...}. It keeps the buffer to clear
// it after the call.
func (p *secretParams) MarshalJSON() ([]byte, error) {
	id, err := json.Marshal(p.ID)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(id)+2*len(p.Value)+24)
	b = append(b, `{"id":`...)
	b = append(b, id...)
	b = append(b, `,"value":`...)
	b = appendJSONString(b, p.Value)
	b = append(b, '}')
	p.buf = b
	return b, nil
}

func (p *secretParams) wipe() { clear(p.buf); clear(p.Value) }

// appendJSONString appends s as a JSON string (UTF-8 is kept; control
// characters, quote and backslash are escaped).
func appendJSONString(b, s []byte) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for _, c := range s {
		switch {
		case c == '"' || c == '\\':
			b = append(b, '\\', c)
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

// SecretStatus calls secret.status: id, set, hint, updated_at. It never
// contains the Secret value.
func (a *API) SecretStatus(id string) (wire.SecretStatusResult, error) {
	c, cancel := ctx()
	defer cancel()
	var r wire.SecretStatusResult
	err := a.m.call(c, func(cl *client.Client) error { var e error; r, e = cl.SecretStatus(c, id); return e })
	return r, wrap(err)
}
