#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# vlt-dev-init.sh: development set-up for vltd on Linux (VLT-IS-003 TASK
# 03-00-00-008). Run as root. This is not a package.
#
# 1. Makes the system groups vlt and vltd and the system user vltd.
# 2. Makes a 32-byte DEK from /dev/urandom and encrypts it with
#    "systemd-creds encrypt --name=vlt-dek --with-key=auto" to
#    /etc/credstore.encrypted/vlt-dek. The plain DEK goes through a pipe
#    only. It is never written to disk. --with-key=null is never used.
# 3. Writes the key mode (host or host+tpm2) to /etc/vltd/key-mode.
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
