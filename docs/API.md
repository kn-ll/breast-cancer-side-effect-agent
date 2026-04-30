# API

默认服务地址：

```text
http://localhost:8080
```

## Health

```bash
curl -sS http://localhost:8080/api/healthz
```

响应：

```json
{
  "ai_enabled": false,
  "ai_model": "deepseek-v4-flash",
  "ai_provider": "deepseek",
  "now": "2026-04-30T10:00:00Z",
  "rule_version": "breast-side-effect-rules-v0.1.0",
  "status": "ok"
}
```

## 提交评估

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我今天发热 38.5 度，还有寒战"}'
```

响应核心字段：

```json
{
  "assessment": {
    "id": "asm_xxx",
    "user_id": "demo-user",
    "risk_level": "high",
    "status": "open",
    "generated_at": "2026-04-30T10:00:00Z",
    "rule_version": "breast-side-effect-rules-v0.1.0",
    "advice": {
      "risk_level": "high",
      "contact_team": true,
      "urgency": "immediate_or_24h"
    },
    "evidence": {
      "matched_rule_id": "H002_HIGH_FEVER_OR_INFECTION",
      "matched_rule_name": "高热或感染风险",
      "matched_keywords": ["寒战", "38.5°C"]
    }
  },
  "needs_follow_up": false
}
```

## 动态追问

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我有点发烧"}'
```

响应会包含：

```json
{
  "needs_follow_up": true,
  "follow_up_questions": [
    "最高体温是多少？是否达到或超过 38°C？",
    "这些症状从什么时候开始，持续了多久？"
  ]
}
```

补充回答：

```bash
curl -sS -X POST http://localhost:8080/api/assessments \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo-user","description":"我有点发烧","follow_up_answers":{"user_answer":"最高 38.3，昨晚开始，还有寒战"}}'
```

## 获取结果

```bash
curl -sS http://localhost:8080/api/assessments/asm_xxx
```

## 获取历史

```bash
curl -sS 'http://localhost:8080/api/history?user_id=demo-user'
```

## 创建协同请求

```bash
curl -sS -X POST http://localhost:8080/api/assessments/asm_xxx/contact-requests \
  -H 'Content-Type: application/json' \
  -d '{"channel":"care_team","message":"用户希望联系团队"}'
```

响应：

```json
{
  "contact_request": {
    "id": "ctr_xxx",
    "assessment_id": "asm_xxx",
    "status": "open",
    "channel": "care_team",
    "handoff_summary": "患者报告..."
  }
}
```

## 关闭评估

```bash
curl -sS -X POST http://localhost:8080/api/assessments/asm_xxx/close \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## 事件埋点

```bash
curl -sS -X POST http://localhost:8080/api/events \
  -H 'Content-Type: application/json' \
  -d '{"assessment_id":"asm_xxx","user_id":"demo-user","event_type":"result_viewed","metadata":{"page":"result"}}'
```

核心事件：

- `assessment_started`
- `assessment_submitted`
- `result_viewed`
- `contact_team_clicked`
- `assessment_closed`

## 规则优化建议

```bash
curl -sS -X POST http://localhost:8080/api/rule-suggestions \
  -H 'Content-Type: application/json' \
  -d '{}'
```
