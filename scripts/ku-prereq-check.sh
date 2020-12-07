#!/bin/sh

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
# Inport logmsg function
. ${KU_DIR}/scripts/ku-lib.sh

while getopts 'fn' o
do
    case $o in
        f)
            fw_satisfies "4.13.12638"
            exit $?
            ;;
        n)
            ndb_installed
            exit $?
            ;;
    esac
done
