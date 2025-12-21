#!/bin/bash
# Docker Controller 功能测试脚本

set -e

CONTROLLER_PORT=8080
CONTROLLER_BIN="./docker-controller"
CONTROLLER_LOG="/tmp/docker-controller-test.log"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}清理资源...${NC}"
    if [ ! -z "$CONTROLLER_PID" ]; then
        kill $CONTROLLER_PID 2>/dev/null || true
        wait $CONTROLLER_PID 2>/dev/null || true
    fi
    # 清理测试容器
    docker ps -a --filter "label=aenv.managed=true" --format "{{.Names}}" | xargs -r docker rm -f 2>/dev/null || true
    echo -e "${GREEN}清理完成${NC}"
}
trap cleanup EXIT

# 检查 Docker
echo -e "${BLUE}=== 检查 Docker 环境 ===${NC}"
if ! docker ps > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker 守护进程未运行或无法访问${NC}"
    echo "请确保 Docker 已启动: docker ps"
    exit 1
fi
echo -e "${GREEN}✓ Docker 守护进程运行正常${NC}"

# 检查 Go 环境
echo -e "\n${BLUE}=== 检查 Go 环境 ===${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: Go 未安装${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go 版本: $(go version)${NC}"

# 下载依赖
echo -e "\n${BLUE}=== 下载依赖 ===${NC}"
cd "$(dirname "$0")"
go mod download 2>&1 | grep -v "^go:" || true
go mod tidy 2>&1 | grep -v "^go:" || true
echo -e "${GREEN}✓ 依赖下载完成${NC}"

# 构建
echo -e "\n${BLUE}=== 构建 Docker Controller ===${NC}"
if go build -o "$CONTROLLER_BIN" ./cmd/main.go 2>&1; then
    echo -e "${GREEN}✓ 构建成功${NC}"
else
    echo -e "${RED}✗ 构建失败${NC}"
    exit 1
fi

# 启动 Controller
echo -e "\n${BLUE}=== 启动 Docker Controller ===${NC}"
"$CONTROLLER_BIN" --server-port=$CONTROLLER_PORT > "$CONTROLLER_LOG" 2>&1 &
CONTROLLER_PID=$!

# 等待启动
sleep 3

# 检查是否启动成功
if ! ps -p $CONTROLLER_PID > /dev/null; then
    echo -e "${RED}✗ Controller 启动失败${NC}"
    echo "日志:"
    cat "$CONTROLLER_LOG"
    exit 1
fi
echo -e "${GREEN}✓ Controller 已启动 (PID: $CONTROLLER_PID, Port: $CONTROLLER_PORT)${NC}"

# 测试函数
test_endpoint() {
    local name=$1
    local method=$2
    local url=$3
    local data=$4
    
    echo -e "\n${YELLOW}测试: $name${NC}"
    
    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
            -H "Content-Type: application/json" \
            -d "$data")
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    echo "响应: $body"
    echo "HTTP 状态码: $http_code"
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
        echo -e "${GREEN}✓ $name 通过${NC}"
        return 0
    else
        echo -e "${RED}✗ $name 失败: HTTP $http_code${NC}"
        return 1
    fi
}

# 测试 1: 健康检查
test_endpoint "健康检查" "GET" "http://localhost:$CONTROLLER_PORT/healthz" ""

# 测试 2: 就绪检查
test_endpoint "就绪检查" "GET" "http://localhost:$CONTROLLER_PORT/readyz" ""

# 测试 3: 创建容器
echo -e "\n${YELLOW}测试: 创建容器${NC}"
CREATE_DATA='{
  "name": "test-env",
  "version": "1.0.0",
  "artifacts": [
    {
      "type": "image",
      "content": "nginx:alpine"
    }
  ],
  "deployConfig": {
    "cpu": "1C",
    "memory": "128M",
    "ttl": "10m",
    "environmentVariables": {
      "TEST_VAR": "test_value"
    }
  }
}'

CREATE_RESPONSE=$(curl -s -X POST "http://localhost:$CONTROLLER_PORT/pods" \
  -H "Content-Type: application/json" \
  -d "$CREATE_DATA")

echo "响应: $CREATE_RESPONSE"

CONTAINER_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$CONTAINER_ID" ]; then
    echo -e "${RED}✗ 创建容器失败: 无法获取容器 ID${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 容器创建成功: $CONTAINER_ID${NC}"

# 等待容器启动
sleep 2

# 验证容器存在
if docker ps --format "{{.Names}}" | grep -q "^${CONTAINER_ID}$"; then
    echo -e "${GREEN}✓ 容器在 Docker 中运行${NC}"
else
    echo -e "${YELLOW}⚠ 容器可能未运行，检查所有容器...${NC}"
    docker ps -a --filter "name=$CONTAINER_ID"
fi

# 测试 4: 获取容器信息
test_endpoint "获取容器信息" "GET" "http://localhost:$CONTROLLER_PORT/pods/$CONTAINER_ID" ""

# 测试 5: 列出容器
test_endpoint "列出容器" "GET" "http://localhost:$CONTROLLER_PORT/pods" ""

# 测试 6: 删除容器
test_endpoint "删除容器" "DELETE" "http://localhost:$CONTROLLER_PORT/pods/$CONTAINER_ID" ""

# 等待删除完成
sleep 1

# 验证容器已删除
if docker ps -a --format "{{.Names}}" | grep -q "^${CONTAINER_ID}$"; then
    echo -e "${YELLOW}⚠ 容器仍在 Docker 中（可能正在删除）${NC}"
else
    echo -e "${GREEN}✓ 容器已从 Docker 中删除${NC}"
fi

echo -e "\n${GREEN}=== 所有测试通过! ===${NC}"

