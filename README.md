# SimpleAgentServert

## 环境要求
- CentOS 7 (glibc 2.17)
- 无 sudo/容器权限
- Go 1.4+（用于编译 Go 程序）

## 架构设计

```
┌─────────────────┐     HTTP API      ┌─────────────────┐
│   Local AI      │ ◄───────────────► │   CentOS 7      │
│   Agent         │   POST /execute   │   Agent Server  │
│   (Trae/Claude) │   GET /health     │   (Go binary)   │
└─────────────────┘                   └─────────────────┘
```

## 快速部署

### 1. 在本地（物理机）交叉编译

macOS/Linux/Windows 任意平台都可以：

```bash
# 克隆项目
git clone https://github.com/lccup/SimpleAgentServert.git
cd SimpleAgentServert

# 交叉编译（静态链接，无需 glibc）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/agent_server agent_server.go
```

### 2. 上传到 CentOS 7

```bash
scp bin/agent_server user@your-centos7:/tmp/
ssh user@your-centos7 "chmod +x /tmp/agent_server"
```

### 3. 后台运行

```bash
# 方法 A：nohup
nohup /tmp/agent_server -port 8080 -apikey your-secret-key > /var/log/agent_server.log 2>&1 &

# 方法 B：systemd（可选）
sudo tee /etc/systemd/system/agent-server.service << 'EOF'
[Unit]
Description=SimpleAgentServert

[Service]
ExecStart=/tmp/agent_server -port 8080 -apikey your-secret-key
Restart=always

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl enable agent-server
sudo systemctl start agent-server
```

## API 使用

### 健康检查

```bash
curl -H "X-API-Key: your-secret-key" http://your-centos7:8080/health
```

**Response:**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "timestamp": 1778256914
}
```

### 执行命令

```bash
curl -X POST http://your-centos7:8080/execute \
  -H "X-API-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"command": "ls -la /home", "timeout": 30}'
```

**Request:**
```json
{
  "command": "ls -la /home",
  "timeout": 30
}
```

**Response:**
```json
{
  "success": true,
  "stdout": "total 64\ndrwxr-xr-x 24 user user 4096 May  8 10:00 .",
  "stderr": "",
  "exit_code": 0,
  "timestamp": 1778256914
}
```

### Python 客户端示例

```python
import requests

resp = requests.post(
    "http://your-centos7:8080/execute",
    headers={"X-API-Key": "your-secret-key"},
    json={"command": "df -h", "timeout": 30}
)
result = resp.json()
print(result["stdout"])
```

## 安全措施

1. **API Key 认证** - 必须通过 `X-API-Key` header 或 `?apikey=` query 参数
2. **命令白名单** - 可选，使用 `-allowlist "git,ls,cd"` 限制可执行命令
3. **超时控制** - 默认 30 秒，最大 300 秒
4. **日志审计** - 所有请求都会记录到日志

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-port` | 监听端口 | 8080 |
| `-apikey` | API 密钥 | 环境变量 AGENT_API_KEY |
| `-allowlist` | 允许的命令列表（逗号分隔） | 空（允许所有） |

## 已在容器中验证

Docker 容器 `centos7-local-test` 测试通过：

```
glibc: 2.17

测试结果:
✅ Go 二进制静态链接（ldd: not a dynamic executable）
✅ /health 接口正常
✅ /execute 接口正常
✅ 命令执行返回正确
```

## 故障排除

**Q: Go 二进制在 CentOS 7 上运行报错 "cannot execute binary file"**
A: 确保使用 `CGO_ENABLED=0` 交叉编译，且 `GOARCH=amd64`

**Q: 服务启动成功但无法访问**
A: 检查防火墙：`iptables -L -n` 或 `firewall-cmd --list-ports`