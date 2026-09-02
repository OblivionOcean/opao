#!/bin/bash
# 更新 dev/go.mod 中 opao 的版本到最新 tag

set -e

# 获取最新版本号（排除 pre-release 版本）
LATEST=$(git tag -l 'v*' | grep -v '-' | sort -V | tail -n1)

if [ -z "$LATEST" ]; then
    echo "Error: No version tags found"
    exit 1
fi

echo "Latest version: $LATEST"

# 更新 go.mod
sed -i "s|github.com/OblivionOcean/opao/v2 v[0-9.]*|github.com/OblivionOcean/opao/v2 $LATEST|g" go.mod

echo "Updated go.mod to use version $LATEST"

# 运行 go mod tidy
go mod tidy

echo "Done!"
