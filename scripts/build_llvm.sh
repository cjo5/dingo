#!/bin/bash

# Based on: https://github.com/llvm/llvm-project/blob/release/10.x/llvm/bindings/go/build.sh

if [ -z "$COMP_DIR" ]; then
    echo "COMP_DIR not set"
    exit 1
fi

if [ -z "$LLVM_DIR" ]; then
    echo "LLVM_DIR not set"
    exit 1
fi

BUILD_OPTS="-DCMAKE_BUILD_TYPE=Debug -DLLVM_TARGETS_TO_BUILD=host -DBUILD_SHARED_LIBS=ON"
BUILD_DIR="$LLVM_DIR/llvm/build/debug"

mkdir -p "$BUILD_DIR"
(cd "$BUILD_DIR" && cmake "$LLVM_DIR/llvm" $BUILD_OPTS)
make -C "$BUILD_DIR" -j4

LLVM_GO="$BUILD_DIR/bin/llvm-go"
BINDINGS_DIR="$LLVM_DIR/llvm/bindings/go/llvm"

$LLVM_GO print-config > "$BINDINGS_DIR/llvm_config.go"
(cd "$BINDINGS_DIR" && go mod init llvm.org/llvm/bindings/go/llvm)

echo "$BUILD_DIR/lib" > "$COMP_DIR/LLVM-LIBS"
