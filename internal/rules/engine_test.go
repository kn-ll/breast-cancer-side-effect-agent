package rules

import (
	"testing"

	"breast-cancer-side-effect-agent/internal/domain"
)

func TestEvaluateHighRiskFever(t *testing.T) {
	engine := NewEngine()
	advice, evidence, source := engine.Evaluate("我今天发热 38.5 度，还有寒战", domain.AIAnalysis{})
	if advice.RiskLevel != domain.RiskHigh {
		t.Fatalf("risk = %s, want high", advice.RiskLevel)
	}
	if evidence.MatchedRuleID != "H002_HIGH_FEVER_OR_INFECTION" {
		t.Fatalf("rule = %s", evidence.MatchedRuleID)
	}
	if source.Version != Version {
		t.Fatalf("version = %s, want %s", source.Version, Version)
	}
}

func TestEvaluateMediumRiskGI(t *testing.T) {
	engine := NewEngine()
	advice, evidence, _ := engine.Evaluate("恶心，腹泻两次，但还能喝水", domain.AIAnalysis{})
	if advice.RiskLevel != domain.RiskMedium {
		t.Fatalf("risk = %s, want medium", advice.RiskLevel)
	}
	if evidence.MatchedRuleID != "M001_GI_SKIN_NEURO" {
		t.Fatalf("rule = %s", evidence.MatchedRuleID)
	}
}

func TestEvaluateLowRiskMildEffects(t *testing.T) {
	engine := NewEngine()
	advice, evidence, _ := engine.Evaluate("最近轻微脱发，有点乏力", domain.AIAnalysis{})
	if advice.RiskLevel != domain.RiskLow {
		t.Fatalf("risk = %s, want low", advice.RiskLevel)
	}
	if evidence.MatchedRuleID != "L001_MILD_EXPECTED_EFFECTS" {
		t.Fatalf("rule = %s", evidence.MatchedRuleID)
	}
}
