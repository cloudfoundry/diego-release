#!/usr/bin/env bash

rm $GOPATH/bin/protoc-gen-gogoslick
go install -v github.com/gogo/protobuf/protoc-gen-gogoslick

protoc --proto_path=../loggregator-api/v2 --gogoslick_out=plugins=grpc:. ../loggregator-api/v2/*.proto
