package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"breast-cancer-side-effect-agent/internal/ai"
	"breast-cancer-side-effect-agent/internal/domain"
	"breast-cancer-side-effect-agent/internal/rules"
	"breast-cancer-side-effect-agent/internal/store"
)

func TestSubmitAssessmentAPI(t *testing.T) {
	fs, err := store.NewFileStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(fs, ai.NewOfflineAnalyzer(), rules.NewEngine(), "static").Routes()

	body, _ := json.Marshal(domain.AssessmentRequest{
		UserID:      "demo-user",
		Description: "我今天发热 38.5 度，还有寒战",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/assessments", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp domain.AssessmentResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Assessment.RiskLevel != domain.RiskHigh {
		t.Fatalf("risk = %s, want high", resp.Assessment.RiskLevel)
	}
	if resp.Assessment.Evidence.MatchedRuleID != "H002_HIGH_FEVER_OR_INFECTION" {
		t.Fatalf("rule = %s", resp.Assessment.Evidence.MatchedRuleID)
	}
}
