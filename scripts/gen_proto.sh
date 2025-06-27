#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Define the base directory of the project (assuming the script is in /scripts)
BASE_DIR=$(dirname "$0")/..

# Define proto paths
PROTO_DIR="$BASE_DIR/proto"
OUTPUT_DIR_GO="$BASE_DIR/core_engine/proto/gen"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR_GO"

# Check if protoc is installed
if ! command -v protoc &> /dev/null
then
    echo "protoc could not be found. Please install Protocol Buffers compiler."
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null
then
    echo "protoc-gen-go could not be found. Please install it with 'go install google.golang.org/protobuf/cmd/protoc-gen-go@latest'"
    exit 1
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null
then
    echo "protoc-gen-go-grpc could not be found. Please install it with 'go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest'"
    exit 1
fi

echo "Generating Go code from Protobuf definitions..."

protoc --proto_path="$PROTO_DIR" \
       --go_out="$OUTPUT_DIR_GO" --go_opt=paths=source_relative \
       --go-grpc_out="$OUTPUT_DIR_GO" --go-grpc_opt=paths=source_relative \
       "$PROTO_DIR/v_architect_core.proto"

echo "Protobuf Go code generation complete."
echo "Generated files are in: $OUTPUT_DIR_GO"

# List generated files (optional, for verification)
ls -l "$OUTPUT_DIR_GO"
