// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package main

func devInstall(env) error   { return errWindowsOnly }
func devUninstall(env) error { return errWindowsOnly }
