#!/bin/bash

echo "Building for mobile (Android/iOS)..."

# 编译 Android AAR
echo "Building Android AAR..."
gomobile bind -tags mobile -target=android -o p2pshare.aar .

# 编译 iOS Framework (需要 macOS)
# echo "Building iOS Framework..."
# gomobile bind -tags mobile -target=ios -o P2PShare.xcframework .

echo "Mobile build completed."