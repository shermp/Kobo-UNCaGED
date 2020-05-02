#!/bin/sh

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/kobo-uncaged
KU_LOG=${KU_DIR}/ku_error.log

cd ${KU_DIR}

$KU_BIN > $KU_LOG
KU_RES=$?

cd -

# Disable the loopback interface when we are done
ip link set lo down
