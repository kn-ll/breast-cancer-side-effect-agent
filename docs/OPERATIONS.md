# Operations

## 环境要求

- Go 1.22+
- Git
- 可选：OpenAI-compatible API key

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
| `OPENAI_API_KEY` | 空 | 配置后启用远程 AI |
| `OPENAI_MODEL` | `gpt-4.1-mini` | 模型名 |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base |

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
