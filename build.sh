#!/bin/bash
set -e

echo "Generating protobuf code"
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/rpc/pb/intercom.proto

echo "Building for Raspberry Pi Zero W"
GOOS=linux GOARCH=arm GOARM=6 go build -o compiled/gointercom_arm6 ./cmd/...
echo "Created gointercom_arm6 binary"
echo "Building for Raspberry Pi 4"
GOOS=linux GOARCH=arm GOARM=7 go build -o compiled/gointercom_arm7 ./cmd/...
echo "Created gointercom_arm7 binary"
echo Complete

#echo "Building server for Raspberry Pi Zero W"
#GOOS=linux GOARCH=arm GOARM=6 go build -o compiled/gointercom-server_arm6 ./server/
#echo "Created gointercom-server_arm6 binary"
#echo "Building for Raspberry Pi 4"
#GOOS=linux GOARCH=arm GOARM=7 go build -o compiled/gointercom-server_arm7 ./server/
#echo "Created gointercom-server_arm7 binary"
#echo Complete
#echo "Building client for Raspberry Pi Zero W"
#GOOS=linux GOARCH=arm GOARM=6 go build -o compiled/gointercom-client_arm6 ./client/
#echo "Created gointercom-client_arm6 binary"
#echo "Building for Raspberry Pi 4"
#GOOS=linux GOARCH=arm GOARM=7 go build -o compiled/gointercom-client_arm7 ./client/
#echo "Created gointercom-client_arm7 binary"
#echo Complete
