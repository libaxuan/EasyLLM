#!/bin/bash
# Build script for EasyLLM
set -e

echo "=== Building EasyLLM ==="

# 1. Build frontend
echo "Building frontend..."
cd web
npm install --legacy-peer-deps
npm run build
cd ..

# 2. Build Go backend
echo "Building Go backend..."
CGO_ENABLED=1 go build -ldflags="-w -s" -o easyllm .

echo ""
echo "=== Build Complete ==="
echo "Binary: ./easyllm"
echo "Run:    ./easyllm"
echo "Or:     cp .env.example .env && ./easyllm"
