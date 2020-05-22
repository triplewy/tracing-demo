#!/bin/bash -e

PATH=$PATH:$GOPATH/bin
protodir=../../pb

protoc --go_out=plugins=grpc:. -I $protodir $protodir/service.proto
