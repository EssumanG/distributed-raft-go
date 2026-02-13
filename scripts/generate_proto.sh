#!/bin/bash

# Script to generate Go code from protobuf definitions

echo "Generating protobuf code ..."

#Install protoc-gen-go and protoc-gen-go-grpc if not already installed
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

#Generate the code
echo "Generate Go code from proto files ..."
protoc -I .. --go_out=.. --go_opt=paths=source_relative \
    --go-grpc_out=.. --go-grpc_opt=paths=source_relative \
    ../internal/rpc/concensus.proto

if [ $? -eq 0 ]; then
    echo "protobuf code generatoed successfully!"
    echo ""
    echo "Generated files:"
    echo "  - rpc/consensus.pb.go"
    echo "  - rpc/consensus_grpc.pb.go"
else
    echo "Failed to generate protobuf code"
    exit 1
fi

