#!/bin/bash

# 本地构建并发布到 GitHub Release

set -e

VERSION=""
PRERELEASE=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        --prerelease)
            PRERELEASE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 -v <version> [--prerelease]"
            echo "Example: $0 -v v1.3.21"
            exit 1
            ;;
    esac
done

if [ -z "$VERSION" ]; then
    echo "Error: Version is required"
    echo "Usage: $0 -v <version> [--prerelease]"
    exit 1
fi

echo "=========================================="
echo "Local Build and Release to GitHub"
echo "=========================================="
echo "Version: $VERSION"
echo "Prerelease: $PRERELEASE"
echo ""

# 1. 清理旧的构建
echo "1. Cleaning old builds..."
rm -rf gocron-package gocron-node-package gocron-build gocron-node-build
echo "✓ Cleaned"
echo ""

# 2. 构建前端
echo "2. Building frontend..."
cd web/vue
yarn install --frozen-lockfile
yarn run build
cd ../..
echo "✓ Frontend built"
echo ""

# 3. 生成静态资源
echo "3. Generating static assets..."
go install github.com/rakyll/statik@latest
go generate ./...
echo "✓ Static assets generated"
echo ""

# 4. 构建所有平台的 gocron-node
echo "4. Building gocron-node for all platforms..."
./package.sh -p "linux,darwin,windows" -a "amd64,arm64" -v "$VERSION"
echo "✓ All gocron-node packages built"
echo ""

# 5. 显示构建结果
echo "5. Build summary:"
echo ""
echo "gocron packages:"
ls -lh gocron-package/
echo ""
echo "gocron-node packages:"
ls -lh gocron-node-package/
echo ""

# 6. 验证包内容
echo "6. Verifying package contents..."
SAMPLE_PACKAGE=$(ls gocron-package/*.tar.gz 2>/dev/null | head -1)
if [ -n "$SAMPLE_PACKAGE" ]; then
    echo "Checking: $SAMPLE_PACKAGE"
    tar tzf "$SAMPLE_PACKAGE" | grep gocron-node-package | head -5
    echo "✓ Packages contain gocron-node-package"
else
    SAMPLE_PACKAGE=$(ls gocron-package/*.zip 2>/dev/null | head -1)
    if [ -n "$SAMPLE_PACKAGE" ]; then
        echo "Checking: $SAMPLE_PACKAGE"
        unzip -l "$SAMPLE_PACKAGE" | grep gocron-node-package | head -5
        echo "✓ Packages contain gocron-node-package"
    fi
fi
echo ""

# 7. 创建 Git tag
echo "7. Creating Git tag..."
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "Tag $VERSION already exists"
    read -p "Delete and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "$VERSION"
        git push origin ":refs/tags/$VERSION" 2>/dev/null || true
    else
        echo "Skipping tag creation"
    fi
fi

if ! git rev-parse "$VERSION" >/dev/null 2>&1; then
    git tag -a "$VERSION" -m "Release $VERSION"
    git push origin "$VERSION"
    echo "✓ Tag created and pushed"
else
    echo "✓ Using existing tag"
fi
echo ""

# 8. 创建 GitHub Release
echo "8. Creating GitHub Release..."
echo ""

PRERELEASE_FLAG=""
if [ "$PRERELEASE" = true ]; then
    PRERELEASE_FLAG="--prerelease"
fi

# 生成 release notes
cat > /tmp/release_notes.md <<EOF
## 🚀 Release $VERSION

### Features
- Cross-platform agent auto-registration support
- Each gocron package includes gocron-node for all platforms

### Packages
- **gocron**: Main application (includes all platform gocron-node packages)
- **gocron-node**: Agent for task execution (standalone packages)

### Supported Platforms
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Installation
Download the appropriate package for your platform and extract it.

For agent auto-registration, see documentation.
EOF

# 检查 gh CLI 是否安装
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed"
    echo "Install it from: https://cli.github.com/"
    echo ""
    echo "Packages are ready in:"
    echo "  - gocron-package/"
    echo "  - gocron-node-package/"
    echo ""
    echo "You can manually create a release on GitHub and upload these files."
    exit 1
fi

# 创建 release
gh release create "$VERSION" \
    --title "Release $VERSION" \
    --notes-file /tmp/release_notes.md \
    $PRERELEASE_FLAG \
    gocron-package/*.tar.gz \
    gocron-package/*.zip \
    gocron-node-package/*.tar.gz \
    gocron-node-package/*.zip

echo ""
echo "=========================================="
echo "✓ Release $VERSION created successfully!"
echo "=========================================="
echo ""
echo "View release: https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/releases/tag/$VERSION"
