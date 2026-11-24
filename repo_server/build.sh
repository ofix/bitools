#!/bin/bash
set -e

APP_NAME="repo-server"
LDFLAGS="-s -w"

echo "🚀 开始跨平台编译..."

# 1. macOS (Intel & Apple Silicon)
echo "🍎 编译 macOS..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o dist/${APP_NAME}-darwin-amd64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o dist/${APP_NAME}-darwin-arm64 .

# 2. Windows
echo "🪟 编译 Windows..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o dist/${APP_NAME}-windows-amd64.exe .

# 3. Linux
echo "🐧 编译 Linux..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o dist/${APP_NAME}-linux-amd64 .

echo "✅ 编译完成！产物在 dist/ 目录下。"