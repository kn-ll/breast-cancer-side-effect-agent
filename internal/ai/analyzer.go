package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"breast-cancer-side-effect-agent/internal/domain"
)

type Analyzer struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewAnalyzerFromEnv() *Analyzer {
	baseURL := strings.TrimRight(os.Getenv("OPENAI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &Analyzer{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func NewOfflineAnalyzer() *Analyzer {
	return &Analyzer{
		model: "heuristic-fallback",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (a *Analyzer) Enabled() bool {
	return a != nil && a.apiKey != ""
}

func (a *Analyzer) Analyze(ctx context.Context, req domain.AssessmentRequest) domain.AIAnalysis {
	fallback := fallbackAnalyze(req)
	if !a.Enabled() {
		return fallback
	}

	payload := fmt.Sprintf(symptomExtractionUserPrompt, req.Description, formatFollowUps(req.FollowUpAnswers))
	var parsed struct {
		Summary            string   `json:"summary"`
		Symptoms           []string `json:"symptoms"`
		TemperatureCelsius *float64 `json:"temperature_celsius"`
		Duration           string   `json:"duration"`
		SeveritySignals    []string `json:"severity_signals"`
		MissingFields      []string `json:"missing_fields"`
		FollowUpQuestions  []string `json:"follow_up_questions"`
	}
	if err := a.chatJSON(ctx, symptomExtractionSystemPrompt, payload, &parsed); err != nil {
		fallback.SafetyWarnings = append(fallback.SafetyWarnings, "remote_ai_unavailable:"+err.Error())
		return fallback
	}

	result := domain.AIAnalysis{
		Summary:            firstNonEmpty(parsed.Summary, fallback.Summary),
		Symptoms:           dedupe(append(parsed.Symptoms, fallback.Symptoms...)),
		TemperatureCelsius: parsed.TemperatureCelsius,
		Duration:           firstNonEmpty(parsed.Duration, fallback.Duration),
		SeveritySignals:    dedupe(append(parsed.SeveritySignals, fallback.SeveritySignals...)),
		MissingFields:      dedupe(parsed.MissingFields),
		FollowUpQuestions:  limitStrings(parsed.FollowUpQuestions, 3),
		GeneratedBy:        "openai-compatible:" + a.model,
		GeneratedAt:        time.Now().UTC(),
	}
	if result.TemperatureCelsius == nil {
		result.TemperatureCelsius = fallback.TemperatureCelsius
	}
	if len(result.MissingFields) == 0 {
		result.MissingFields = fallback.MissingFields
	}
	if len(result.FollowUpQuestions) == 0 {
		result.FollowUpQuestions = fallback.FollowUpQuestions
	}
	return result
}

func (a *Analyzer) GenerateUserExplanation(ctx context.Context, assessment domain.Assessment) (string, []string) {
	fallback := safeFallbackExplanation(
		assessment.RiskLevel,
		assessment.Evidence.MatchedRuleID,
		assessment.Evidence.MatchedRuleName,
		assessment.Evidence.Reason,
	)
	if !a.Enabled() {
		return fallback, reviewSafety(fallback, assessment.RiskLevel)
	}

	prompt := fmt.Sprintf(
		explanationPrompt,
		assessment.RiskLevel,
		assessment.Evidence.MatchedRuleID,
		assessment.Evidence.MatchedRuleName,
		strings.Join(assessment.Evidence.MatchedKeywords, ", "),
		strings.Join(assessment.Advice.NextSteps, "；"),
	)
	text, err := a.chatText(ctx, "你只输出安全、审计友好的中文解释。", prompt)
	if err != nil {
		return fallback, []string{"remote_explanation_unavailable:" + err.Error()}
	}
	warnings := reviewSafety(text, assessment.RiskLevel)
	if len(warnings) > 0 {
		return fallback, warnings
	}
	return strings.TrimSpace(text), nil
}

func (a *Analyzer) GenerateHandoffSummary(ctx context.Context, assessment domain.Assessment) (string, []string) {
	fallback := fmt.Sprintf(
		"患者报告：%s。系统判定风险等级为 %s，命中规则 %s（%s）。建议：%s",
		assessment.AIAnalysis.Summary,
		assessment.RiskLevel,
		assessment.Evidence.MatchedRuleID,
		assessment.Evidence.MatchedRuleName,
		strings.Join(assessment.Advice.NextSteps, "；"),
	)
	if !a.Enabled() {
		return fallback, reviewSafety(fallback, assessment.RiskLevel)
	}

	prompt := fmt.Sprintf(
		handoffPrompt,
		assessment.Description,
		assessment.AIAnalysis.Summary,
		assessment.RiskLevel,
		assessment.Evidence.MatchedRuleID,
		assessment.Evidence.MatchedRuleName,
		strings.Join(assessment.Advice.NextSteps, "；"),
	)
	text, err := a.chatText(ctx, "你只输出安全、简洁、面向护理团队的中文交接摘要。", prompt)
	if err != nil {
		return fallback, []string{"remote_handoff_unavailable:" + err.Error()}
	}
	warnings := reviewSafety(text, assessment.RiskLevel)
	if len(warnings) > 0 {
		return fallback, warnings
	}
	return strings.TrimSpace(text), nil
}

func (a *Analyzer) GenerateRuleImprovementSuggestion(assessments []domain.Assessment, events []domain.EventLog, ruleVersion string) domain.RuleImprovementSuggestion {
	highCount := 0
	contactCount := 0
	ruleHits := map[string]int{}
	for _, item := range assessments {
		if item.RiskLevel == domain.RiskHigh {
			highCount++
		}
		ruleHits[item.Evidence.MatchedRuleID]++
	}
	for _, event := range events {
		if event.EventType == domain.EventContactTeamClicked {
			contactCount++
		}
	}
	topRule := "none"
	topCount := 0
	for ruleID, count := range ruleHits {
		if count > topCount {
			topRule = ruleID
			topCount = count
		}
	}
	return domain.RuleImprovementSuggestion{
		ID:                   domain.NewID("sug"),
		GeneratedAt:          time.Now().UTC(),
		RuleVersion:          ruleVersion,
		Observation:          fmt.Sprintf("当前共有 %d 次评估，%d 次高风险，%d 次点击联系团队；最高频命中规则为 %s（%d 次）。", len(assessments), highCount, contactCount, topRule, topCount),
		Suggestion:           "建议由临床/护理负责人复盘高频规则和联系团队行为，确认是否需要调整关键词、追问字段或升级阈值。原型不自动发布新规则。",
		RequiresHumanReview:  true,
		SupportingEventCount: len(events),
	}
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (a *Analyzer) chatJSON(ctx context.Context, system string, user string, target any) error {
	content, err := a.chatText(ctx, system, user)
	if err != nil {
		return err
	}
	jsonText := extractJSONObject(content)
	if jsonText == "" {
		return errors.New("model did not return a JSON object")
	}
	return json.Unmarshal([]byte(jsonText), target)
}

func (a *Analyzer) chatText(ctx context.Context, system string, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("chat completion status %d: %s", resp.StatusCode, string(raw))
	}

	var decoded chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

func fallbackAnalyze(req domain.AssessmentRequest) domain.AIAnalysis {
	text := req.Description + " " + formatFollowUps(req.FollowUpAnswers)
	lower := strings.ToLower(text)
	symptomMap := map[string]string{
		"发热": "fever", "发烧": "fever", "低烧": "fever", "高烧": "fever", "高热": "fever",
		"寒战": "chills",
		"恶心": "nausea",
		"呕吐": "vomiting", "吐": "vomiting",
		"腹泻": "diarrhea", "拉肚子": "diarrhea",
		"皮疹": "rash", "红疹": "rash",
		"口腔溃疡": "mouth_sore", "嘴破": "mouth_sore",
		"手脚麻": "neuropathy", "手脚麻木": "neuropathy",
		"胸痛":   "chest_pain",
		"呼吸困难": "shortness_of_breath", "喘不过气": "shortness_of_breath",
		"乏力": "fatigue", "疲劳": "fatigue",
		"脱发": "hair_loss",
		"头晕": "dizziness",
		"肿胀": "swelling", "水肿": "swelling",
	}
	var symptoms []string
	var signals []string
	for zh, normalized := range symptomMap {
		if strings.Contains(lower, strings.ToLower(zh)) {
			symptoms = append(symptoms, normalized)
		}
	}
	temps := extractTemperatures(text)
	var temp *float64
	if len(temps) > 0 {
		sort.Float64s(temps)
		value := temps[len(temps)-1]
		temp = &value
		if value >= 38 {
			signals = append(signals, "fever_38_plus")
		} else if value >= 37.5 {
			signals = append(signals, "fever_37_5_to_38")
		}
	}
	if strings.Contains(text, "寒战") {
		signals = append(signals, "chills", "possible_infection")
	}
	if strings.Contains(text, "感染") || strings.Contains(text, "化脓") || strings.Contains(text, "红肿热痛") {
		signals = append(signals, "possible_infection")
	}
	if strings.Contains(text, "持续呕吐") || strings.Contains(text, "一直吐") {
		signals = append(signals, "persistent_vomiting")
	}
	if strings.Contains(text, "不能喝水") || strings.Contains(text, "无法喝水") || strings.Contains(text, "尿很少") || strings.Contains(text, "尿量明显减少") {
		signals = append(signals, "dehydration")
	}
	if strings.Contains(text, "呼吸困难") || strings.Contains(text, "胸痛") || strings.Contains(text, "喘不过气") {
		signals = append(signals, "breathing_distress")
	}

	missing := missingFields(text, temp)
	questions := followUpQuestions(missing, text)
	return domain.AIAnalysis{
		Summary:            "用户描述：" + truncate(strings.TrimSpace(req.Description), 80),
		Symptoms:           dedupe(symptoms),
		TemperatureCelsius: temp,
		Duration:           extractDuration(text),
		SeveritySignals:    dedupe(signals),
		MissingFields:      missing,
		FollowUpQuestions:  questions,
		GeneratedBy:        "heuristic-fallback",
		GeneratedAt:        time.Now().UTC(),
	}
}

func missingFields(text string, temp *float64) []string {
	lower := strings.ToLower(text)
	var missing []string
	hasFeverWord := strings.Contains(text, "发烧") || strings.Contains(text, "发热") || strings.Contains(text, "低烧") || strings.Contains(text, "高烧")
	if hasFeverWord && temp == nil {
		missing = append(missing, "temperature_celsius")
	}
	if !strings.Contains(text, "今天") && !strings.Contains(text, "昨天") && !strings.Contains(text, "昨晚") && !strings.Contains(text, "小时") && !strings.Contains(text, "天") {
		missing = append(missing, "duration")
	}
	if (strings.Contains(text, "呕吐") || strings.Contains(text, "腹泻") || strings.Contains(text, "拉肚子")) &&
		!strings.Contains(text, "喝水") && !strings.Contains(lower, "hydration") {
		missing = append(missing, "hydration_status")
	}
	if !strings.Contains(text, "化疗") && !strings.Contains(text, "放疗") && !strings.Contains(text, "内分泌") && !strings.Contains(text, "靶向") && !strings.Contains(text, "免疫") {
		missing = append(missing, "treatment_context")
	}
	return dedupe(missing)
}

func followUpQuestions(missing []string, text string) []string {
	var questions []string
	for _, field := range missing {
		switch field {
		case "temperature_celsius":
			questions = append(questions, "最高体温是多少？是否达到或超过 38°C？")
		case "duration":
			questions = append(questions, "这些症状从什么时候开始，持续了多久？")
		case "hydration_status":
			questions = append(questions, "现在是否能正常喝水？尿量是否明显减少？")
		case "treatment_context":
			questions = append(questions, "最近正在接受哪类治疗，是否刚完成化疗、放疗、靶向或内分泌治疗？")
		}
	}
	if strings.Contains(text, "发") && !strings.Contains(text, "寒战") {
		questions = append(questions, "是否伴随寒战、胸痛、呼吸困难、伤口红肿或化脓？")
	}
	return limitStrings(dedupe(questions), 3)
}

func formatFollowUps(values map[string]string) string {
	if len(values) == 0 {
		return "无"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, key+": "+values[key])
	}
	return strings.Join(parts, "；")
}

var tempPattern = regexp.MustCompile(`([3-4][0-9](?:\.\d+)?)\s*(?:°c|℃|度|c)?`)

func extractTemperatures(text string) []float64 {
	matches := tempPattern.FindAllStringSubmatch(strings.ToLower(text), -1)
	var temps []float64
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		if v >= 35 && v <= 43 {
			temps = append(temps, v)
		}
	}
	return temps
}

func extractDuration(text string) string {
	markers := []string{"今天", "昨天", "昨晚", "前天"}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return marker
		}
	}
	re := regexp.MustCompile(`\d+\s*(小时|天|周)`)
	if match := re.FindString(text); match != "" {
		return match
	}
	return ""
}

func dedupe(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func limitStrings(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func extractJSONObject(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return ""
	}
	return content[start : end+1]
}
