#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# uninstall.sh: removes the portable install of VoltioVolt (VLT-IS-008
# TASK 03-00-00-006). No sudo. Asks first. Removes the install folder,
# the Start menu shortcut and the autostart shortcut of the user. Asks
# separately before it deletes the data folder.
set -eu
DEST="${HOME}/.local/opt/voltiovolt"
DESKTOP="${HOME}/.local/share/applications/voltiovolt.desktop"
AUTOSTART="${HOME}/.config/autostart/voltiovolt.desktop"
DATA1="${DEST}/data"
DATA2="${HOME}/.local/share/voltiovolt"

echo "This removes the VoltioVolt files from ${DEST}."
echo "Close VoltioVolt before you continue."
printf "Continue? (Y/N) "
read -r ANS || ANS=
if [ "${ANS}" != "Y" ] && [ "${ANS}" != "y" ]; then
	echo "Cancelled."
	exit 0
fi

if [ -f "${DESKTOP}" ]; then
	rm -f "${DESKTOP}"
	echo "Removed the Start menu shortcut."
fi
if [ -f "${AUTOSTART}" ]; then
	rm -f "${AUTOSTART}"
	echo "Removed the autostart shortcut."
fi

if [ -f "${DEST}/voltiovolt" ]; then rm -f "${DEST}/voltiovolt"; fi
if [ -f "${DEST}/vlt-probe" ]; then rm -f "${DEST}/vlt-probe"; fi
if [ -f "${DEST}/vlt-simulator" ]; then rm -f "${DEST}/vlt-simulator"; fi

DATADIR=
if [ -d "${DATA1}" ]; then
	DATADIR=${DATA1}
elif [ -d "${DATA2}" ]; then
	DATADIR=${DATA2}
fi
if [ -n "${DATADIR}" ]; then
	echo
	echo "The data folder is ${DATADIR}."
	echo "If you delete it, the stored Settings and Secrets are lost."
	printf "Delete the data folder? (Y/N) "
	read -r ANS2 || ANS2=
	if [ "${ANS2}" = "Y" ] || [ "${ANS2}" = "y" ]; then
		rm -rf "${DATADIR}"
		echo "Data folder deleted."
	else
		echo "Data folder kept."
	fi
fi

if [ -d "${DEST}" ]; then
	rmdir "${DEST}" 2>/dev/null || echo "The folder ${DEST} is not empty. It is kept."
fi

echo "Done."
