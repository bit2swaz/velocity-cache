



PACKAGE_PATH="./cmd/velocity"

OUTPUT_DIR="build"

BINARY_NAME="velocity-cli"



echo "==> Cleaning old builds..."
rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"




echo "==> Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64" "${PACKAGE_PATH}"
echo "==> Compressing Linux AMD64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-linux-amd64"


echo "==> Building macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64" "${PACKAGE_PATH}"
echo "==> Compressing macOS AMD64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-darwin-amd64"


echo "==> Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-darwin-arm64" "${PACKAGE_PATH}"
echo "==> Compressing macOS ARM64..."
tar -czf "${OUTPUT_DIR}/${BINARY_NAME}-darwin-arm64.tar.gz" -C "${OUTPUT_DIR}" "${BINARY_NAME}-darwin-arm64"


echo "==> Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe" "${PACKAGE_PATH}"
echo "==> Compressing Windows AMD64..."
zip "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.zip" -j "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe" 









echo "==> Build complete! Compressed binaries are in the '${OUTPUT_DIR}' directory."