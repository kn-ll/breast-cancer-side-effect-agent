# Rules and Audit

## 规则版本

```text
breast-side-effect-rules-v0.1.0
```

规则定义在：

```text
internal/rules/engine.go
```

## 风险分层

### 高风险

| Rule ID | 名称 | 命中条件 | 建议 |
| --- | --- | --- | --- |
| `H001_BREATHING_OR_CHEST` | 呼吸或胸痛急症风险 | 呼吸困难、胸痛、喘不过气、严重气短 | 立即线下就医 / 24h 联系团队 |
| `H002_HIGH_FEVER_OR_INFECTION` | 高热或感染风险 | 体温 >= 38°C、寒战、感染、化脓、红肿热痛 | 立即线下就医 / 24h 联系团队 |
| `H003_ALLERGY_BLEEDING_NEURO` | 严重过敏、出血或神经系统风险 | 严重过敏、脸肿、嘴唇肿、出血不止、意识模糊、晕厥 | 立即线下就医 |
| `H004_DEHYDRATION_OR_PERSISTENT_VOMITING` | 持续呕吐或脱水风险 | 持续呕吐、无法喝水、尿量明显减少 | 尽快线下就医 / 联系团队 |

### 中风险

| Rule ID | 名称 | 命中条件 | 建议 |
| --- | --- | --- | --- |
| `M002_MILD_FEVER_OR_WORSENING_PAIN` | 低热或症状加重 | 37.5°C <= 体温 < 38°C、疼痛加重、乏力加重 | 联系团队或密切观察 |
| `M001_GI_SKIN_NEURO` | 胃肠道、皮肤或神经症状 | 恶心、呕吐、腹泻、皮疹、口腔溃疡、手脚麻木、头晕 | 联系团队或密切观察 |
| `M003_LYMPHEDEMA_OR_SWELLING` | 肿胀或疑似淋巴水肿 | 手臂肿胀、腋窝肿胀、乳房肿胀、淋巴水肿 | 联系团队评估 |

### 低风险

| Rule ID | 名称 | 命中条件 | 建议 |
| --- | --- | --- | --- |
| `L001_MILD_EXPECTED_EFFECTS` | 轻微常见治疗相关反应 | 轻微乏力、轻微脱发、食欲下降、睡眠差、潮热 | 继续观察与记录 |
| `LOW_DEFAULT_OBSERVE` | 未命中明确风险规则 | 未命中高/中/低显式规则 | 继续观察，症状加重时重新评估 |

## 审计字段

每次 assessment 必须包含：

```json
{
  "risk_level": "high",
  "generated_at": "2026-04-30T10:00:00Z",
  "rule_version": "breast-side-effect-rules-v0.1.0",
  "evidence": {
    "matched_rule_id": "H002_HIGH_FEVER_OR_INFECTION",
    "matched_rule_name": "高热或感染风险",
    "matched_keywords": ["寒战", "38.5°C"],
    "reason": "体温或感染相关信号达到高风险阈值：寒战, 38.5°C。",
    "ai_summary": "用户描述：我今天发热 38.5 度，还有寒战",
    "ai_signals": ["fever_38_plus", "chills", "possible_infection"]
  },
  "rule_source": {
    "engine": "rules",
    "version": "breast-side-effect-rules-v0.1.0",
    "rule_id": "H002_HIGH_FEVER_OR_INFECTION",
    "rule_name": "高热或感染风险",
    "source": "prototype_clinical_safety_rules"
  }
}
```

## 可观测性事件

| Event | 触发点 | 记录位置 |
| --- | --- | --- |
| `assessment_started` | 用户打开输入页 | 前端调用 `/api/events` |
| `assessment_submitted` | 后端保存评估后 | 后端自动写入 |
| `result_viewed` | 用户进入结果页 | 前端调用 `/api/events` |
| `contact_team_clicked` | 用户创建协同请求 | 后端自动写入 |
| `assessment_closed` | 用户关闭评估 | 后端自动写入 |

扩展事件：

- `ai_analysis_started`
- `ai_analysis_completed`
- `followup_question_generated`
- `handoff_summary_generated`

## 规则升级策略

MVP 不允许 AI 自动修改规则。建议流程：

1. 通过 `POST /api/rule-suggestions` 生成历史观察。
2. 临床/护理负责人复盘高频事件和误报。
3. 修改 `internal/rules/engine.go`。
4. 更新 `rules.Version`。
5. 补充或更新测试。
6. 发布新版本。
