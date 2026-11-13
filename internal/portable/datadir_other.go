// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !unix && !windows

package portable

func profileDir() (string, error) { return "data", nil }
