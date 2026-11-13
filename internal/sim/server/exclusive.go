// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import "errors"

// ErrAlreadyRunning is returned when a second vlt-simulator tries to
// listen on the endpoint of OD-44.
var ErrAlreadyRunning = errors.New("the simulator already runs")
