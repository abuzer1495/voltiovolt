# Source register (SEC-060)

This register lists each source that the VLT project uses for protocol facts, test vectors, tools and text. The project uses facts only. It copies no code and no text from other UPS software.

| No. | Source | URL | Used for | Use | Date read |
|-----|--------|-----|----------|-----|-----------|
| R-01 | Megatec protocol document (Network UPS Tools web site) | https://networkupstools.org/protocols/megatec.html | Q1, F, I formats, status bits, invalid command handling, "D" and "Q" commands | Facts only | 2026-11-10 |
| R-02 | nut-upsuser mailing list, Tunçmatik Newtech Pro 1 kVA (2010), "Source A" | https://lists.alioth.debian.org/pipermail/nut-upsuser/2010-February/005856.html | Test vectors and field meanings of the extended Dialect (family sample) | Facts and vectors only. The real QID value is not used. | 2026-11-10 |
| R-03 | Gist ao-kenji/7070940, "Source B" | https://gist.github.com/ao-kenji/7070940 | Q1 vector of a standby UPS over USB | Vector only | 2026-11-10 |
| R-04 | Linux kernel documentation: hidraw | https://docs.kernel.org/hid/hidraw.html | Report number byte for read() and write() | Facts only | 2026-11-10 |
| R-05 | Linux kernel documentation: introduction to HID report descriptors | https://docs.kernel.org/hid/hidintro.html | sysfs report_descriptor, item examples | Facts only | 2026-11-10 |
| R-06 | Microsoft documentation: HIDP_CAPS | https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/hidpi/ns-hidpi-_hidp_caps | Report lengths include the report ID | Facts only | 2026-11-10 |
| R-07 | GNU AGPL-3.0 text | https://www.gnu.org/licenses/agpl-3.0.txt | LICENSE file, without changes | Full text | 2026-11-10 |
| R-08 | Contributor Covenant 2.1, official repository EthicalSource/contributor_covenant, branch "release", commit 5794f441d325aac3f5e263c0154c0fe897f8dbd2, file content/version/2/1/code_of_conduct.md | https://github.com/EthicalSource/contributor_covenant | CODE_OF_CONDUCT.md | Full text; only the contact placeholder is changed | 2026-11-10 |
| R-09 | Developer Certificate of Origin | https://developercertificate.org/ | DCO sign-off rule | Reference | 2026-11-10 |
| R-10 | go.bug.st/serial v1.8.0 source (module proxy) | https://pkg.go.dev/go.bug.st/serial | Read timeout behavior (Read returns 0 bytes on timeout) | API facts | 2026-11-10 |
| R-11 | GitHub documentation: download workflow artifacts | https://docs.github.com/en/actions/how-tos/manage-workflow-runs/download-workflow-artifacts | docs/user/probe.md | Facts only | 2026-11-10 |
| R-12 | VLT-REF-001 Rev 00, Owner interoperability reference (OD-24). Copy in docs/reference/VLT-REF-001.md, without a change, SHA-256 4ee716bc84db6752a8385d89c33bae9a83f3e313bec5ff4b8adad39442eeef8e | docs/reference/VLT-REF-001.md | Megatec Q1, F, I frame layout, status bits, CONTROL command formats, synthetic test vectors, Reference UPS connection (virtual COM port) | Behavior facts and synthetic examples only. Accepted source for SEC-060 (OD-24). | 2026-11-10 |

## Owner analysis files (OD-25)

The Owner analysis files are private. VLT does not read, use or commit them. The only accepted result of that analysis is VLT-REF-001. The CI job "gitleaks" fails if a tracked path contains "ANALIZ/" or a tracked file has the extension .gpr, .rep or .gzf (SEC-101).
