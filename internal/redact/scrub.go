// SPDX-License-Identifier: AGPL-3.0-or-later

package redact

import "regexp"

// Text patterns that Scrub removes (VLT-IS-003 TASK 03-00-00-002 step 5).
// The order is important: the "/bot" rule runs before the bare token rule.
var rules = []struct {
	re   *regexp.Regexp
	repl string
}{
	// The path segment "/bot" with a token, for example
	// https://api.telegram.org/bot<token>/getMe.
	{regexp.MustCompile(`/bot[^/\s?#"']+`), "/bot" + Redacted},
	// A bare Telegram bot token: 8 to 10 digits, ":", 35 characters.
	{regexp.MustCompile(`[0-9]{8,10}:[A-Za-z0-9_-]{35}`), Redacted},
	// "Authorization:" header values, to the end of the line.
	{regexp.MustCompile(`(?i)(authorization\s*:\s*)[^\r\n]+`), "${1}" + Redacted},
	// "Bearer" tokens.
	{regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]+`), "${1}" + Redacted},
	// URL query values with these names.
	{regexp.MustCompile(`(?i)([?&](?:access_token|token|key|secret|password)=)[^&#\s"']*`), "${1}" + Redacted},
}

// Scrub returns s without the Secret patterns of VLT-IS-003 TASK 03-00-00-002
// step 5.
func Scrub(s string) string {
	for _, r := range rules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
