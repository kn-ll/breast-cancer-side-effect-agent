package ai

import (
	"strings"

	"breast-cancer-side-effect-agent/internal/domain"
)

func reviewSafety(text string, level domain.RiskLevel) []string {
	lower := strings.ToLower(text)
	var warnings []string
	blocked := []string{"自行停药", "自己停药", "自行换药", "自己换药", "自行加药", "不用联系医生", "不用就医", "肯定没事", "一定没事"}
	for _, phrase := range blocked {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			warnings = append(warnings, "ai_output_contains_blocked_phrase:"+phrase)
		}
	}
	if level == domain.RiskHigh && !strings.Contains(text, "就医") && !strings.Contains(text, "联系") {
		warnings = append(warnings, "high_risk_output_missing_escalation_path")
	}
	return warnings
}

func safeFallbackExplanation(level domain.RiskLevel, ruleID string, ruleName string, reason string) string {
	switch level {
	case domain.RiskHigh:
		return "系统命中高风险规则 " + ruleID + "（" + ruleName + "）。" + reason + " 建议立即线下就医，并在 24 小时内联系治疗团队。"
	case domain.RiskMedium:
		return "系统命中中风险规则 " + ruleID + "（" + ruleName + "）。" + reason + " 建议联系治疗团队或密切观察症状变化。"
	default:
		return "系统命中低风险规则 " + ruleID + "（" + ruleName + "）。" + reason + " 建议继续观察与记录，若加重请重新评估。"
	}
}
