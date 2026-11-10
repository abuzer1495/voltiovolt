// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bufio"
	"strings"
)

func bufioReader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }
