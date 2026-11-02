#!/bin/bash

# linux_device_hyprland.sh
# Reports the active window title on Linux (Hyprland)
# by: @inoryxin

# --- config start
URL="http://10.0.0.123:9010/api/device/set" # server URL, ends with /api/device/set
SECRET="114514" # your secret
DEVICE_ID="desktop" # unique device id
DEVICE_SHOW_NAME="My PC" # name shown on the page
# --- config end

LASTWINDOW="inoryxin" # dummy start value, so an idle desktop after boot counts as not in use

while true; do

    HYPRCTL_BIN=$(which hyprctl)

    PACKAGE_NAME=$($HYPRCTL_BIN activewindow | grep "title:" | sed 's/title://g' | sed 's/^[[:space:]]*//')

    echo "$PACKAGE_NAME"

    if [ "$PACKAGE_NAME" = "" ]; then
        STATUS=false
    else
        STATUS=true
    fi

    echo "$STATUS"

    json_data='{
    "secret": "'$SECRET'",
    "id": "'$DEVICE_ID'",
    "show_name": "'$DEVICE_SHOW_NAME'",
    "using": '$STATUS',
    "status": "'$PACKAGE_NAME'"
}'

    if [ "$PACKAGE_NAME" = "$LASTWINDOW" ]; then
        echo "window not change, bypass"
    else
        curl -X POST "$URL" -H "Content-Type: application/json" -d "$json_data"
    fi

    LASTWINDOW="$PACKAGE_NAME"

    sleep 10
done
