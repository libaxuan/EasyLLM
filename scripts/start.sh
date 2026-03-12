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

# ── 禁用已知会抢占端口的系统代理服务 ────────────────────────────────────────
disable_conflicting_services() {
  # LVSecurityAgent 的 manageproxy 组件会占用 8021，需要在启动前阻止
  local proxy_plist="/Library/LaunchAgents/com.lvmagent.manageproxy.plist"
  if [ -f "${proxy_plist}" ]; then
    local svc_label="com.lvmagent.manageproxy"
    # 禁用（防止重启后自动加载）
    launchctl disable "gui/$(id -u)/${svc_label}" 2>/dev/null || true
    # 卸载当前会话中的服务
    launchctl unload "${proxy_plist}" 2>/dev/null || true
    # 杀掉进程（包括可能的孤儿进程）
    pkill -9 -f "dvc-manageproxy-exe" 2>/dev/null || true
    echo -e "${YELLOW}⚠  已停止系统代理服务 (${svc_label})${RESET}"
    sleep 1
  fi
}

# ── 杀占用端口的进程 ──────────────────────────────────────────────────────────
kill_port() {
  local pids any_killed=0

  # 1. 杀所有持有该端口的进程（含 go run 父进程和编译后的子进程）
  pids=$(lsof -ti TCP:"${PORT}" 2>/dev/null || true)
  if [ -n "${pids}" ]; then
    echo -e "${YELLOW}⚠  端口 ${PORT} 被占用 (PID: $(echo $pids | tr '\n' ' '))，正在终止...${RESET}"
    echo "${pids}" | xargs kill -9 2>/dev/null || true
    any_killed=1
  fi

  # 2. 额外杀掉 go run main.go 及其衍生的 easyllm 编译产物（孤儿进程）
  pkill -9 -f "easyllm" 2>/dev/null && any_killed=1 || true
  pkill -9 -f "go-build.*easyllm" 2>/dev/null || true

  if [ "${any_killed}" -eq 1 ]; then
    sleep 1  # 等待端口完全释放
  fi

  # 3. 确认端口已释放（检测仍有 ghost socket 时给出提示）
  local can_bind=0
  python3 -c "
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
try:
    s.bind(('0.0.0.0', ${PORT})); s.listen(1); s.close(); exit(0)
except: exit(1)
" 2>/dev/null && can_bind=1 || can_bind=0

  if [ "${can_bind}" -eq 0 ]; then
    echo -e "${RED}✗  端口 ${PORT} 仍被系统内核 socket 占用（ghost socket）${RESET}"
    echo -e "${YELLOW}   解决方案：重启 Mac 后重新运行此脚本${RESET}"
    exit 1
  fi
  echo -e "${GREEN}✓  端口 ${PORT} 已释放${RESET}"
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
disable_conflicting_services
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
