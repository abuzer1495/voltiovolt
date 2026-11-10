#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# install-test.sh: test install of VoltioVolt on Linux (VLT-IS-007 TASK
# 03-00-00-001). This is not a package. Use a test computer. Run as root,
# with sudo (then your user goes into the group vlt):
#
#     sudo sh install-test.sh
#
# 1. Copies vltd, vlt-gui and vlt-probe to /usr/local/lib/voltiovolt-test/.
# 2. Runs packaging/linux/vlt-dev-init.sh (users, groups, encrypted DEK,
#    key mode, polkit rule).
# 3. Installs vltd.service and vltd-fallback.service with the new path.
# 4. Starts vltd.

set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "install-test.sh: run as root: sudo sh install-test.sh" >&2
	exit 1
fi
here="$(cd "$(dirname "$0")" && pwd)"
dest=/usr/local/lib/voltiovolt-test

for f in vltd vlt-gui vlt-probe packaging/linux/vlt-dev-init.sh packaging/linux/50-vltd.rules packaging/linux/vltd.service packaging/linux/vltd-fallback.service; do
	if [ ! -f "$here/$f" ]; then
		echo "install-test.sh: $f not found in $here; extract all files of the tar.gz file first" >&2
		exit 1
	fi
done
if systemctl is-active --quiet vltd 2>/dev/null; then
	echo "install-test.sh: vltd runs; stop it first (or run uninstall-test.sh)" >&2
	exit 1
fi

install -d -m 0755 -o root -g root "$dest"
for f in vltd vlt-gui vlt-probe; do
	install -m 0755 -o root -g root "$here/$f" "$dest/$f"
done
echo "install-test.sh: binaries copied to $dest"

sh "$here/packaging/linux/vlt-dev-init.sh"

for u in vltd.service vltd-fallback.service; do
	sed "s|^ExecStart=/usr/local/bin/vltd\$|ExecStart=$dest/vltd|" "$here/packaging/linux/$u" >"/etc/systemd/system/$u.new"
	chmod 0644 "/etc/systemd/system/$u.new"
	mv "/etc/systemd/system/$u.new" "/etc/systemd/system/$u"
done
if ! grep -q "^ExecStart=$dest/vltd\$" /etc/systemd/system/vltd.service; then
	echo "install-test.sh: ExecStart not set in vltd.service" >&2
	exit 1
fi
systemctl daemon-reload
systemctl start vltd
systemctl --no-pager status vltd || true

echo
echo "Done. Log out and log in again (group vlt). Then start the VLT GUI: $dest/vlt-gui"
