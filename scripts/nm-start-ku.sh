#!/bin/sh

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/kobo-uncaged
FBINK_BIN=${KU_DIR}/bin/fbink
KU_LOG=${KU_DIR}/ku_error.log

# Inport logmsg function
. ${KU_DIR}/scripts/ku-lib.sh

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up
logmsg "I" "Enabled loopback interface"

# Check if we have a ku.toml file. If not, copy it from the default file
if [ ! -f "${KU_DIR}/config/ku.toml" ]; then
    logmsg "I" "No existing config file. Using default."
    cp -f "${KU_DIR}/config/ku.toml.default" "${KU_DIR}/config/ku.toml"
fi

cd ${KU_DIR}

logmsg "I" "Starting Kobo UNCaGED"
$KU_BIN > $KU_LOG
KU_RES=$?

cd -

# Disable the loopback interface when we are done
ip link set lo down
logmsg "I" "Disabled loopback interface"
