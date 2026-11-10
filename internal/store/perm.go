// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "errors"

// ErrPermissions is the base error of the permission checks. vltd stops when
// a check gives this error (VLT-IS-003 TASK 03-00-00-003 step 6.3).
var ErrPermissions = errors.New("store: permissions are less strict than required")

// DefaultDataDir returns the data directory of this platform:
// Linux /var/lib/vltd, Windows %ProgramData%\VoltioVolt.
func DefaultDataDir() string { return defaultDataDir() }
