#!/bin/bash
set -e

echo "Building for Raspberry Pi Zero W"
GOOS=linux GOARCH=arm GOARM=6 go build -o gointercom-arm6
echo "Created gointercom-arm6 binary"
echo "Building for Raspberry Pi 4"
GOOS=linux GOARCH=arm GOARM=7 go build -o gointercom-arm7
echo "Created gointercom-arm7 binary"
echo Complete
