#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# uninstall-test.sh: removes the test install of VoltioVolt (VLT-IS-007
# TASK 03-00-00-001). Run as root: sudo sh uninstall-test.sh
# Asks before it deletes the data (/var/lib/vltd, the DEK and /etc/vltd).

set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "uninstall-test.sh: run as root: sudo sh uninstall-test.sh" >&2
	exit 1
fi
dest=/usr/local/lib/voltiovolt-test

systemctl stop vltd vltd-fallback 2>/dev/null || true
rm -f /etc/systemd/system/vltd.service /etc/systemd/system/vltd-fallback.service /etc/polkit-1/rules.d/50-vltd.rules
systemctl daemon-reload
systemctl reset-failed vltd vltd-fallback 2>/dev/null || true
rm -rf "$dest"
echo "uninstall-test.sh: service units, polkit rule and $dest removed"

printf 'Delete all VLT data, the Secrets and the DEK (/var/lib/vltd, /etc/credstore.encrypted/vlt-dek, /etc/vltd)? Type YES to delete: '
read -r answer || answer=""
if [ "$answer" = "YES" ]; then
	rm -rf /var/lib/vltd /etc/vltd
	rm -f /etc/credstore.encrypted/vlt-dek
	echo "uninstall-test.sh: data deleted"
else
	echo "uninstall-test.sh: data kept"
fi
echo "The users and groups vltd and vlt are not removed. To remove them: userdel vltd; groupdel vltd; groupdel vlt"
echo "Done."
