#!/bin/sh

KU_DIR=.adds/kobo-uncaged
KU_TMP_DIR=/tmp/ku

# Copy essential programs and scripts to /tmp, to ensure they will be available
# during USBMS session
mkdir -p "$KU_TMP_DIR"
cp "/mnt/onboard/${KU_DIR}/scripts/run-ku.sh" "$KU_TMP_DIR"
cp "/mnt/onboard/${KU_DIR}/scripts/nickel-usbms.sh" "$KU_TMP_DIR"
cp "/mnt/onboard/${KU_DIR}/bin/button_scan" "$KU_TMP_DIR"
cp "/mnt/onboard/${KU_DIR}/bin/fbink" "$KU_TMP_DIR"

cd "$KU_TMP_DIR" || exit 255
exec ./run-ku.sh "$KU_DIR" "$KU_TMP_DIR"
