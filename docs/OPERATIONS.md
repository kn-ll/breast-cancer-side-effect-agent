# Operations

## 环境要求

- Go 1.22+
- Git
- 可选：DeepSeek API key

## 本地运行

```bash
cd /Users/xingjun.liu/work/test/breast-cancer-side-effect-agent
go run ./cmd/server
```

打开：

```text
http://localhost:8080
```

## 配置项

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 端口 |
| `STORE_PATH` | `data/store.json` | 本地 JSON 存储文件 |
| `STATIC_DIR` | `internal/httpapi/static` | 前端静态文件目录 |
| `DEEPSEEK_API_KEY` | 空 | 配置后启用 DeepSeek AI |
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | DeepSeek 模型名 |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | DeepSeek OpenAI-compatible API base |
| `DEEPSEEK_THINKING` | `disabled` | DeepSeek 思考模式开关，原型默认非思考模式 |
| `DEEPSEEK_REASONING_EFFORT` | `high` | 当 `DEEPSEEK_THINKING=enabled` 时生效 |
| `OPENAI_API_KEY` | 空 | 兼容后备：没有 `DEEPSEEK_API_KEY` 时读取 |
| `OPENAI_MODEL` | `gpt-4.1-mini` | 兼容后备模型名 |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | 兼容后备 API base |

## DeepSeek 接入

按照 DeepSeek 官方文档：

- OpenAI 格式 base URL：`https://api.deepseek.com`
- 鉴权：`Authorization: Bearer ${DEEPSEEK_API_KEY}`
- 模型：`deepseek-v4-flash`
- JSON Output：请求体设置 `response_format: {"type":"json_object"}`，并在 prompt 中给出 JSON 输出样例

启动：

```bash
export DEEPSEEK_API_KEY="your_deepseek_api_key"
export DEEPSEEK_MODEL="deepseek-v4-flash"
export DEEPSEEK_BASE_URL="https://api.deepseek.com"
export DEEPSEEK_THINKING="disabled"
go run ./cmd/server
```

检查：

```bash
curl -sS http://localhost:8080/api/healthz
```

期望看到：

```json
{
  "ai_enabled": true,
  "ai_provider": "deepseek",
  "ai_model": "deepseek-v4-flash"
}
```

## 测试和构建

```bash
go test ./...
go build ./cmd/server
```

## 初始化 Git 仓库

目标远端：

```text
https://github.com/kn-ll/breast-cancer-side-effect-agent.git
```

完整命令：

```bash
cd /Users/xingjun.liu/work/test/breast-cancer-side-effect-agent
git init
git branch -M main
git remote add origin https://github.com/kn-ll/breast-cancer-side-effect-agent.git
git status --short
git add .
git commit -m "Initial breast cancer side effect agent prototype"
git status --short
```

推送：

```bash
git push -u origin main
```

如果 `git push` 要求认证，需要先完成 GitHub 凭证配置，例如 GitHub CLI、credential helper 或 Personal Access Token。

## 验收脚本

启动服务后执行：

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我今天发热 38.5 度，还有寒战"}'
```

期望：

- `risk_level = high`
- `advice.contact_team = true`
- `evidence.matched_rule_id = H002_HIGH_FEVER_OR_INFECTION`
- `rule_version = breast-side-effect-rules-v0.1.0`

中风险：

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我有点恶心，腹泻了两次，但还能喝水"}'
```

低风险：

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我最近轻微脱发，有点乏力"}'
```

动态追问：

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我有点发烧"}'
```

## 存储说明

默认存储文件：

```text
data/store.json
```

包含：

- `assessments`
- `contact_requests`
- `event_logs`
- `rule_improvement_suggestions`

这是原型存储。生产化建议迁移到 PostgreSQL，并给 event log 单独建表。
