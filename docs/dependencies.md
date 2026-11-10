# Dependency register (SEC-061)

Permitted licenses for Go modules in VLT binaries: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. CI checks this list with go-licenses.

## Go modules in VLT binaries

| Name | Version | License | Reason | Used by |
|------|---------|---------|--------|---------|

## CI tools (not in VLT binaries)

| Name | Version | License | Reason |
|------|---------|---------|--------|
| actions/checkout | v7.0.1 (3d3c42e5aac5ba805825da76410c181273ba90b1) | MIT | Check out the repository |
| actions/setup-go | v7.0.0 (b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) | MIT | Install Go 1.27.3 |
| actions/upload-artifact | v7.0.2 (cf430e030ddbb5b0abf93d22962f4752f3646cd9) | MIT | vlt-probe artifacts |
| gitleaks (binary) | v8.30.1 | MIT | Secret scan of the full history (SEC-063) |
| golang.org/x/vuln (govulncheck) | v1.9.0 | BSD-3-Clause | Vulnerability check (SEC-062) |
| github.com/google/go-licenses/v2 | v2.0.1 | Apache-2.0 | License check (SEC-061) |
