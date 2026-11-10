#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# vlt-dev-init.sh: development set-up for vltd on Linux (VLT-IS-003 TASK
# 03-00-00-008). Run as root. This is not a package.
#
# 1. Makes the system groups vlt and vltd and the system user vltd. If
#    SUDO_USER is set (the script runs with sudo), adds that user to the
#    group vlt: members of vlt can use the IPC socket, so the VLT GUI
#    (VLT-IS-005 TASK 03-00-00-009 step 6).
# 2. Makes a 32-byte DEK from /dev/urandom and encrypts it with
#    "systemd-creds encrypt --name=vlt-dek --with-key=auto" to
#    /etc/credstore.encrypted/vlt-dek. The plain DEK goes through a pipe
#    only. It is never written to disk. --with-key=null is never used.
# 3. Writes the key mode (host or host+tpm2) to /etc/vltd/key-mode.
# 4. Copies the polkit rule 50-vltd.rules to /etc/polkit-1/rules.d/ (owner
#    root, mode 0644) (VLT-IS-004 TASK 03-00-00-001 step 4).
#
# An existing /etc/credstore.encrypted/vlt-dek is not changed: a new DEK
# makes all stored Secrets unreadable.

set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "vlt-dev-init.sh: run as root" >&2
	exit 1
fi
if ! command -v systemd-creds >/dev/null 2>&1; then
	echo "vlt-dev-init.sh: systemd-creds not found (systemd 250 or later is necessary)" >&2
	exit 1
fi

getent group vlt >/dev/null || groupadd --system vlt
getent group vltd >/dev/null || groupadd --system vltd
if ! getent passwd vltd >/dev/null; then
	useradd --system --gid vltd --no-create-home --home-dir /var/lib/vltd \
		--shell /usr/sbin/nologin --comment "VoltioVolt service" vltd
fi

# The user who started sudo goes into the group vlt (not root, and only an
# existing user).
if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
	if getent passwd "$SUDO_USER" >/dev/null; then
		if id -nG "$SUDO_USER" | tr ' ' '\n' | grep -qx vlt; then
			echo "vlt-dev-init.sh: $SUDO_USER is a member of the group vlt"
		else
			usermod -a -G vlt "$SUDO_USER"
			echo "vlt-dev-init.sh: $SUDO_USER added to the group vlt; log out and log in again to use the VLT GUI"
		fi
	else
		echo "vlt-dev-init.sh: SUDO_USER=$SUDO_USER is not a user; no group change" >&2
	fi
fi

install -d -m 0700 -o root -g root /etc/credstore.encrypted
cred=/etc/credstore.encrypted/vlt-dek
if [ -e "$cred" ]; then
	echo "vlt-dev-init.sh: $cred exists; it is not changed"
else
	umask 077
	tmp="$cred.new"
	# 32 random bytes, through a pipe, to the encrypted credential file.
	head -c 32 /dev/urandom | systemd-creds encrypt --name=vlt-dek --with-key=auto - "$tmp"
	chmod 0600 "$tmp"
	mv "$tmp" "$cred"
	echo "vlt-dev-init.sh: $cred made"
fi

# Key mode. systemd-creds(1): with --with-key=auto, the TPM2 key is used if
# a TPM2 device is found and not running in a container; the host key is
# used if /var/lib/systemd/ is on persistent media. "systemd-analyze
# has-tpm2" exits with 0 only for a usable TPM2 (systemd-analyze(1)).
if systemd-analyze has-tpm2 --quiet >/dev/null 2>&1 &&
	! systemd-detect-virt --container --quiet; then
	mode="host+tpm2"
else
	mode="host"
fi
install -d -m 0755 -o root -g root /etc/vltd
printf '%s\n' "$mode" >/etc/vltd/key-mode.new
chown root:root /etc/vltd/key-mode.new
chmod 0644 /etc/vltd/key-mode.new
mv /etc/vltd/key-mode.new /etc/vltd/key-mode
echo "vlt-dev-init.sh: key mode $mode written to /etc/vltd/key-mode"
if [ "$mode" = "host" ]; then
	echo "vlt-dev-init.sh: no TPM2: the Vault state will be \"reduced protection\" (SEC-005)"
fi

# polkit rule (power off and hibernate for the user vltd only).
here="$(cd "$(dirname "$0")" && pwd)"
if [ -d /etc/polkit-1/rules.d ] || install -d -m 0755 -o root -g root /etc/polkit-1/rules.d; then
	install -m 0644 -o root -g root "$here/50-vltd.rules" /etc/polkit-1/rules.d/50-vltd.rules
	echo "vlt-dev-init.sh: polkit rule installed: /etc/polkit-1/rules.d/50-vltd.rules"
fi
