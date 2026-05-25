#!/bin/bash
# Build engine.wasm per plan §11.2, §13.1
# GOOS=js GOARCH=wasm go build produces ~15 MB WASM module

set -e

cd "$(dirname "$0")"

echo "Building engine.wasm (GOOS=js GOARCH=wasm)..."

# Set GOOS/GOARCH for WASM build
export GOOS=js
export GOARCH=wasm

# Build the WASM binary
go build -o ../dist/engine.wasm .

# Copy the wasm_exec.js helper from Go SDK
GOROOT=$(go env GOROOT)
# Try both paths for different Go versions
if [ -f "${GOROOT}/misc/wasm/wasm_exec.js" ]; then
    cp "${GOROOT}/misc/wasm/wasm_exec.js" ../dist/
elif [ -f "${GOROOT}/lib/wasm/wasm_exec.js" ]; then
    cp "${GOROOT}/lib/wasm/wasm_exec.js" ../dist/
else
    echo "Warning: wasm_exec.js not found, skipping copy"
fi

echo "Built wasm/engine -> dist/engine.wasm"
echo "Size: $(wc -c < ../dist/engine.wasm) bytes"

# Verify the file is a valid WASM module (check magic number)
if [ "$(head -c 4 ../dist/engine.wasm)" = $'\0asm' ]; then
    echo "Verified: Valid WASM magic number"
else
    echo "Warning: Output may not be a valid WASM module"
fi

echo "WASM engine build complete"
