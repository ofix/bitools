#!/bin/bash

# 设置输出目录
OUTPUT_DIR="dist"
mkdir -p $OUTPUT_DIR

echo "开始一键跨平台编译"

# 禁用 CGO，确保纯静态编译，避免跨平台时的 C 库依赖问题
export CGO_ENABLED=0

# 1. 编译 Windows 64位
echo "📦 正在编译 Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o $OUTPUT_DIR/p2pshare.windows.amd64.exe .

# 2. 编译 macOS Intel 64位
echo "📦 正在编译 macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o $OUTPUT_DIR/p2pshare.darwin.amd64 .

# 3. 编译 macOS Apple Silicon (M1/M2/M3)
echo "📦 正在编译 macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -o $OUTPUT_DIR/p2pshare.darwin.arm64 .

# 4. 编译 Linux 64位
echo "📦 正在编译 Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o $OUTPUT_DIR/p2pshare.linux.amd64 .

# 5. 编译 Linux ARM64 (如树莓派、ARM服务器)
echo "📦 正在编译 Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o $OUTPUT_DIR/p2pshare.linux.arm64 .

echo "✅ 全部编译完成！产物已输出到 $OUTPUT_DIR 目录："
ls -lh $OUTPUT_DIR