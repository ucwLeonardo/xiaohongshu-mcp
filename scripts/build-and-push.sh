#!/usr/bin/env bash
# After modifying xiaohongshu-mcp, run this to commit, push, and rebuild Docker image.
# Usage: bash scripts/build-and-push.sh [commit message]
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

IMAGE="ucwleonardo/xiaohongshu-mcp:chrome-fix"

# 1. Commit & push
echo "=== [1/3] Git commit & push ==="
git add .
if git diff --cached --quiet; then
  echo "  没有新改动，跳过 commit"
else
  MSG="${1:-feat: update xiaohongshu-mcp}"
  git commit -m "$MSG"
  git push origin main
  echo "  已推送到 origin/main"
fi

# 2. Build Docker image
echo "=== [2/3] 构建 Docker 镜像: $IMAGE ==="
docker build -t "$IMAGE" .

# 3. Restart running MCP containers (if any) to use new image
echo "=== [3/3] 重启使用该镜像的 MCP 容器 ==="
CONTAINERS=$(docker ps --filter "ancestor=$IMAGE" -q 2>/dev/null || true)
if [ -n "$CONTAINERS" ]; then
  echo "$CONTAINERS" | xargs docker restart
  echo "  已重启 $(echo "$CONTAINERS" | wc -l) 个容器"
else
  echo "  没有运行中的 MCP 容器"
fi

echo ""
echo "=== 完成 ==="
