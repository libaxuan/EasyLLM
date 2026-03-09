#!/usr/bin/env bash
# EasyLLM 快速启动脚本 (Mac / Linux)
# 用法:
#   ./scripts/start.sh          # go run 模式（开发）
#   ./scripts/start.sh --build  # 先编译再运行（生产）
#   ./scripts/start.sh --prod   # 直接运行已编译的 ./easyllm 二进制

set -euo pipefail

# ── 颜色 ──────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

# ── 项目根目录（脚本所在目录的上一级）────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# ── 读取端口（优先 .env，其次环境变量，默认 8021）───────────────────────────
PORT="${SERVER_PORT:-8021}"
if [ -f ".env" ]; then
  _P=$(grep -E '^\s*SERVER_PORT\s*=' .env 2>/dev/null | tail -1 | cut -d'=' -f2 | tr -d '[:space:]')
  [ -n "${_P}" ] && PORT="${_P}"
fi

# ── 参数解析 ──────────────────────────────────────────────────────────────────
MODE="dev"   # dev | build | prod
for arg in "$@"; do
  case "$arg" in
    --build) MODE="build" ;;
    --prod)  MODE="prod"  ;;
  esac
done

echo ""
echo -e "${BOLD}${CYAN}╔══════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║        EasyLLM  快速启动脚本        ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════╝${RESET}"
echo -e "  模式  : ${YELLOW}${MODE}${RESET}"
echo -e "  端口  : ${YELLOW}${PORT}${RESET}"
echo -e "  根目录: ${ROOT_DIR}"
echo ""

# ── 杀占用端口的进程 ──────────────────────────────────────────────────────────
kill_port() {
  local pids
  # lsof 在 Mac/Linux 均可用
  pids=$(lsof -ti :"${PORT}" 2>/dev/null || true)
  if [ -n "${pids}" ]; then
    echo -e "${YELLOW}⚠  端口 ${PORT} 被占用 (PID: $(echo $pids | tr '\n' ' '))，正在终止...${RESET}"
    echo "${pids}" | xargs kill -9 2>/dev/null || true
    sleep 1
    echo -e "${GREEN}✓  端口 ${PORT} 已释放${RESET}"
  else
    echo -e "${GREEN}✓  端口 ${PORT} 空闲${RESET}"
  fi
}

# ── 加载 .env ─────────────────────────────────────────────────────────────────
load_env() {
  if [ -f ".env" ]; then
    echo -e "${CYAN}→  加载 .env${RESET}"
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
  else
    echo -e "${YELLOW}→  未找到 .env，使用默认配置（可 cp .env.example .env 自定义）${RESET}"
  fi
}

# ── 构建前端 ──────────────────────────────────────────────────────────────────
build_frontend() {
  echo -e "\n${CYAN}→  构建前端...${RESET}"
  cd web
  npm install --legacy-peer-deps --silent
  npm run build
  cd ..
  echo -e "${GREEN}✓  前端构建完成${RESET}"
}

# ── 构建 Go 二进制 ────────────────────────────────────────────────────────────
build_backend() {
  echo -e "\n${CYAN}→  编译 Go 后端...${RESET}"
  CGO_ENABLED=1 go build -ldflags="-w -s" -o easyllm .
  echo -e "${GREEN}✓  编译完成 → ./easyllm${RESET}"
}

# ── 打印访问信息 ──────────────────────────────────────────────────────────────
print_info() {
  local _port="${SERVER_PORT:-${PORT}}"
  echo ""
  echo -e "${BOLD}${GREEN}═══════════════════════════════════════${RESET}"
  echo -e "${BOLD}${GREEN}  ✓  EasyLLM 已启动${RESET}"
  echo -e "${BOLD}${GREEN}═══════════════════════════════════════${RESET}"
  echo -e "  Web UI : ${CYAN}http://localhost:${_port}${RESET}"
  echo -e "  API    : ${CYAN}http://localhost:${_port}/api/v1${RESET}"
  echo -e "  Pool   : ${CYAN}http://localhost:${_port}/pool/status${RESET}"
  echo -e "${BOLD}${GREEN}═══════════════════════════════════════${RESET}"
  echo ""
  echo -e "  按 ${YELLOW}Ctrl+C${RESET} 停止服务"
  echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
kill_port
load_env

case "${MODE}" in

  # 开发模式：go run（无需预编译）
  dev)
    echo -e "\n${CYAN}→  以 go run 方式启动（开发模式）${RESET}"
    print_info
    go run main.go
    ;;

  # 编译后运行：先 build 前端+后端，再运行二进制
  build)
    build_frontend
    build_backend
    echo -e "\n${CYAN}→  启动编译后的二进制${RESET}"
    print_info
    ./easyllm
    ;;

  # 生产模式：直接运行已有二进制（需提前 build）
  prod)
    if [ ! -f "./easyllm" ]; then
      echo -e "${RED}✗  未找到 ./easyllm，请先运行: ./scripts/start.sh --build${RESET}"
      exit 1
    fi
    echo -e "\n${CYAN}→  启动生产二进制${RESET}"
    print_info
    ./easyllm
    ;;

esac
