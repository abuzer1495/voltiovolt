// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

// CSP is the Content Security Policy of the VLT GUI (SEC-095). The built
// web/index.html has the same text in a meta element; a front-end test and
// TestCSPText check it.
const CSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"
