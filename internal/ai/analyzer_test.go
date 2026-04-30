package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"breast-cancer-side-effect-agent/internal/domain"
)

func TestDeepSeekAnalyzeUsesOfficialChatCompletionShape(t *testing.T) {
	var captured chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"summary\":\"用户发热 38.5°C 并伴寒战。\",\"symptoms\":[\"fever\",\"chills\"],\"temperature_celsius\":38.5,\"duration\":\"今天\",\"severity_signals\":[\"fever_38_plus\",\"chills\"],\"missing_fields\":[],\"follow_up_questions\":[]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("DEEPSEEK_BASE_URL", server.URL)
	t.Setenv("DEEPSEEK_MODEL", "")
	t.Setenv("DEEPSEEK_THINKING", "")
	t.Setenv("OPENAI_API_KEY", "")

	analyzer := NewAnalyzerFromEnv()
	got := analyzer.Analyze(context.Background(), domain.AssessmentRequest{
		UserID:      "demo-user",
		Description: "我今天发热 38.5 度，还有寒战",
	})

	if analyzer.Provider() != "deepseek" {
		t.Fatalf("provider = %s, want deepseek", analyzer.Provider())
	}
	if analyzer.Model() != "deepseek-v4-flash" {
		t.Fatalf("model = %s, want deepseek-v4-flash", analyzer.Model())
	}
	if captured.Model != "deepseek-v4-flash" {
		t.Fatalf("request model = %s", captured.Model)
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "json_object" {
		t.Fatalf("response_format = %#v, want json_object", captured.ResponseFormat)
	}
	if captured.Thinking == nil || captured.Thinking.Type != "disabled" {
		t.Fatalf("thinking = %#v, want disabled", captured.Thinking)
	}
	if got.GeneratedBy != "deepseek:deepseek-v4-flash" {
		t.Fatalf("generated_by = %s", got.GeneratedBy)
	}
	if got.TemperatureCelsius == nil || *got.TemperatureCelsius != 38.5 {
		t.Fatalf("temperature = %#v", got.TemperatureCelsius)
	}
}

func TestAnalyzerDefaultsToDeepSeekFlashWhenNoRemoteKeyIsConfigured(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")

	analyzer := NewAnalyzerFromEnv()
	if analyzer.Enabled() {
		t.Fatalf("analyzer should be disabled without an API key")
	}
	if analyzer.Provider() != "deepseek" {
		t.Fatalf("provider = %s, want deepseek", analyzer.Provider())
	}
	if analyzer.Model() != "deepseek-v4-flash" {
		t.Fatalf("model = %s, want deepseek-v4-flash", analyzer.Model())
	}
}
