#!/bin/bash

# --- Configuration ---
# Path to your main package
PACKAGE_PATH="./cmd/velocity"
# Output directory for builds
OUTPUT_DIR="build"
# Base name for the binary
BINARY_NAME="velocity-cli"
# --- End Configuration ---

# Clean up previous builds
echo "==> Cleaning old builds..."
rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

# --- Build Targets ---

# Linux AMD64
echo "==> Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64" "${PACKAGE_PATH}"
echo "==> Compressing Linux AMD64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-linux-amd64"

# macOS AMD64 (Intel Macs)
echo "==> Building macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64" "${PACKAGE_PATH}"
echo "==> Compressing macOS AMD64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-darwin-amd64"

# macOS ARM64 (Apple Silicon Macs)
echo "==> Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-darwin-arm64" "${PACKAGE_PATH}"
echo "==> Compressing macOS ARM64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-darwin-arm64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-darwin-arm64"

# Windows AMD64
echo "==> Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe" "${PACKAGE_PATH}"
echo "==> Compressing Windows AMD64..."
zip "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.zip" -j "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe" 
# Note: '-j' flag in zip removes the directory structure

# Optional: Clean up the raw binaries after zipping
# echo "==> Cleaning up raw binaries..."
# rm "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64"
# rm "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64"
# rm "${OUTPUT_DIR}/${BINARY_NAME}-darwin-arm64"
# rm "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe"

echo "==> Build complete! Compressed binaries are in the '${OUTPUT_DIR}' directory."