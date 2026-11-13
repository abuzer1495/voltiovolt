#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# install.sh: portable install of VoltioVolt (VLT-IS-008 TASK 03-00-00-006).
# No sudo. Copies voltiovolt, vlt-probe and vlt-simulator to
# ~/.local/opt/voltiovolt. Asks before it makes a Start menu shortcut for
# VoltioVolt (not for the simulator).
set -eu
SRC=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEST="${HOME}/.local/opt/voltiovolt"
DESKTOP_DIR="${HOME}/.local/share/applications"
DESKTOP="${DESKTOP_DIR}/voltiovolt.desktop"

for f in voltiovolt vlt-probe vlt-simulator; do
	if [ ! -f "${SRC}/${f}" ]; then
		echo "ERROR: ${f} is not in ${SRC}. Extract all files of the tar.gz file first." >&2
		exit 1
	fi
done

echo "Copy the files to ${DEST}"
mkdir -p "${DEST}"
install -m 0755 "${SRC}/voltiovolt" "${DEST}/voltiovolt"
install -m 0755 "${SRC}/vlt-probe" "${DEST}/vlt-probe"
install -m 0755 "${SRC}/vlt-simulator" "${DEST}/vlt-simulator"

printf "Make a Start menu shortcut? (Y/N) "
read -r ANS || ANS=
if [ "${ANS}" = "Y" ] || [ "${ANS}" = "y" ]; then
	mkdir -p "${DESKTOP_DIR}"
	cat > "${DESKTOP}" << DESK
[Desktop Entry]
Type=Application
Name=VoltioVolt
Exec="${DEST}/voltiovolt"
Terminal=false
Categories=Utility;
DESK
	chmod 0644 "${DESKTOP}"
	echo "Shortcut: ${DESKTOP}"
fi

echo "Done. Start VoltioVolt: ${DEST}/voltiovolt"
