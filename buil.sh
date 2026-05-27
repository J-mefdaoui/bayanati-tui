#!/bin/bash

# Create dist directory
mkdir -p dist

# Clean previous builds
echo "Cleaning previous builds..."
rm -rf dist/*

# Build for Linux (64-bit)
echo "Building for Linux (64-bit)..."
GOOS=linux GOARCH=amd64 go build -o dist/bayanati-cli-linux-amd64

# Build for Linux (ARM64 - Raspberry Pi, etc.)
#echo "Building for Linux (ARM64)..."
#GOOS=linux GOARCH=arm64 go build -o dist/bayanati-cli-linux-arm64

# Build for Windows (64-bit)
echo "Building for Windows (64-bit)..."
GOOS=windows GOARCH=amd64 go build -o dist/bayanati-cli.exe

# Build for Windows (32-bit)
#echo "Building for Windows (32-bit)..."
#GOOS=windows GOARCH=386 go build -o dist/bayanati-cli-windows-386.exe

# Build for macOS (Intel)
#echo "Building for macOS (Intel)..."
#GOOS=darwin GOARCH=amd64 go build -o dist/bayanati-cli-macos-intel

# Build for macOS (Apple Silicon M1/M2/M3)
e#cho "Building for macOS (Apple Silicon)..."
#GOOS=darwin GOARCH=arm64 go build -o dist/bayanati-cli-macos-arm64

echo ""
echo "✅ All builds complete!"
echo ""
ls -lh dist/
