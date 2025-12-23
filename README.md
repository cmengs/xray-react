# Collector PoC (singbox / xray protocol monitoring)

功能概览
- 接收代理/agent 上报的连接/协议事件（JSON POST /ingest）
- 将事件写入 SQLite（PoC）
- 按协议导出 Prometheus 指标（/metrics）
- 提供简单 API 查询最近连接（/api/v1/connections）
- 支持简单 API token（通过 X-API-Key 头）

快速开始（本地）
1. 构建并运行（需要 Go 1.20+）
    - go build ./cmd/collector
    - API_TOKEN=changeme ./collector

2. 通过 docker-compose 运行
    - docker-compose build
    - docker-compose up -d

3. 验证 metrics 页面
    - 打开 http://localhost:8080/metrics

示例上报 JSON（websocket）:
{
"timestamp": "2025-12-22T12:00:00Z",
"src_ip": "1.2.3.4",
"src_port": 54321,
"dst_ip": "8.8.8.8",
"dst_port": 443,
"protocol": "ws",
"user": "userid123",
"bytes_up": 1024,
"bytes_down": 2048,
"status": "established",
"meta": {"sni":"example.com", "ws_path":"/chat", "ws_subprotocol":"proto"}
}

示例上报（curl）：
- WebSocket 示例

curl -X POST http://localhost:8080/ingest \
-H "Content-Type: application/json" \
-H "X-API-Key: changeme123" \
-d '{
"timestamp":"2025-12-22T12:00:00Z",
"src_ip":"1.2.3.4",
"src_port":54321,
"dst_ip":"8.8.8.8",
"dst_port":443,
"protocol":"ws",
"user":"alice",
"bytes_up":512,
"bytes_down":1024,
"status":"established",
"meta":{"sni":"example.com","ws_path":"/v1/ws"}
}'

- Vmess 示例（常见字段：id/uuid）

curl -X POST http://localhost:8080/ingest \
-H "Content-Type: application/json" \
-H "X-API-Key: changeme123" \
-d '{
"timestamp":"2025-12-22T12:05:00Z",
"src_ip":"10.0.0.2",
"src_port":40000,
"dst_ip":"9.9.9.9",
"dst_port":443,
"protocol":"vmess",
"bytes_up":0,
"bytes_down":0,
"status":"failed",
"reason":"auth_failed",
"meta":{"id":"1111-2222-3333-4444","sni":"vm.example"}
}'

- Trojan 示例

curl -X POST http://localhost:8080/ingest \
-H "Content-Type: application/json" \
-H "X-API-Key: changeme123" \
-d '{
"timestamp":"2025-12-22T12:06:00Z",
"src_ip":"10.0.0.3",
"src_port":40123,
"dst_ip":"9.9.9.9",
"dst_port":443,
"protocol":"trojan",
"status":"failed",
"reason":"auth_failed",
"meta":{"password":"trojan-secret","sni":"trojan.example"}
}'

查询最近连接
- GET http://localhost:8080/api/v1/connections

安全
- 通过设置环境变量 `API_TOKEN` 可强制校验 `X-API-Key` 请求头。生产中请使用更严格的认证（OAuth2 / JWT / mTLS）与 HTTPS。

扩展建议（后续工作）
- 把 SQLite 换成 PostgreSQL / ClickHouse（视历史数据保留与查询需求），并加入分页/过滤 API（按 protocol/user/sni/time）。
- 更完善的 parser：从 singbox/xray 的 access log 或内置 hook 直接映射更多字段（例如 vmess/vless 的 alterId、flow、flow_control 信息）。
- 结合 Prometheus Alertmanager 与 Grafana 模板创建报警与可视化面板。
- 在代理端（singbox/xray）实现侧插件，把握手/认证事件主动上报（减少被动日志解析）。

如何把这些文件推到仓库（示例 Git 操作）

```bash
# 在仓库根目录或新目录执行
git checkout -b feat/collector-poc
# 将上面文件分别保存到相应路径
git add go.mod cmd/collector/main.go internal/collector/collector.go internal/parser/parser.go internal/storage/sqlite.go Dockerfile docker-compose.yml README.md
git commit -m "feat: collector PoC for singbox/xray protocol monitoring"
git push -u origin feat/collector-poc
# 然后在 GitHub 上打开 PR