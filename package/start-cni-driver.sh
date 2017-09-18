#!/bin/bash


if [ "${RANCHER_DEBUG}" == "true" ]; then
    set -x
    DEBUG="--debug"
fi

rancher-cni-driver ${DEBUG}
if [ $? -ne 0 ]; then
    echo "error running rancher-cni-driver"
    exit 1
fi

touch /var/log/rancher-cni.log && exec tail ---disable-inotify -F /var/log/rancher-cni.log
