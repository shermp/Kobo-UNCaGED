#!/bin/sh

KU_DIR=.adds/kobo-uncaged
KU_TMP_DIR=/tmp/ku

# Copy essential programs and scripts to /tmp, to ensure they will be available
# during USBMS session
KU_REQ_FILES="scripts/run-ku.sh scripts/nickel-usbms.sh bin/button_scan bin/fbink"
mkdir -p "$KU_TMP_DIR"
for cur_file in ${KU_REQ_FILES}; do
    cp -f "/mnt/onboard/${KU_DIR}/${cur_file}" "${KU_TMP_DIR}/${cur_file##*/}"
done

# Check if we have a ku.toml file. If not, copy it from the default file
if [ ! -f "/mnt/onboard/${KU_DIR}/config/ku.toml" ]; then
    cp -f "/mnt/onboard/${KU_DIR}/config/ku.toml.default" "/mnt/onboard/${KU_DIR}/config/ku.toml"
fi

cd "$KU_TMP_DIR" || exit 255
./run-ku.sh "$KU_DIR" "$KU_TMP_DIR"

# Cleanup behind us, with gloves on
# (i.e., avoid rm -rf "${KU_TMP_DIR}" to avoid unfortunate mistakes,
# because we're root, and the rootfs is stupidly rw by default on Kobo).
for cur_file in ${KU_REQ_FILES}; do
    rm -f "${KU_TMP_DIR}/${cur_file##*/}"
done
