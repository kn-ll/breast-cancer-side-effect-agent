package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type RiskLevel string

const (
	RiskHigh   RiskLevel = "high"
	RiskMedium RiskLevel = "medium"
	RiskLow    RiskLevel = "low"
)

const (
	AssessmentStatusOpen   = "open"
	AssessmentStatusClosed = "closed"
)

const (
	EventAssessmentStarted   = "assessment_started"
	EventAssessmentSubmitted = "assessment_submitted"
	EventResultViewed        = "result_viewed"
	EventContactTeamClicked  = "contact_team_clicked"
	EventAssessmentClosed    = "assessment_closed"

	EventAIAnalysisStarted       = "ai_analysis_started"
	EventAIAnalysisCompleted     = "ai_analysis_completed"
	EventFollowUpGenerated       = "followup_question_generated"
	EventHandoffSummaryGenerated = "handoff_summary_generated"
)

type AssessmentRequest struct {
	UserID          string            `json:"user_id"`
	Description     string            `json:"description"`
	FollowUpAnswers map[string]string `json:"follow_up_answers,omitempty"`
}

type AssessmentResponse struct {
	Assessment        Assessment `json:"assessment"`
	NeedsFollowUp     bool       `json:"needs_follow_up"`
	FollowUpQuestions []string   `json:"follow_up_questions,omitempty"`
}

type Assessment struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	Description     string            `json:"description"`
	FollowUpAnswers map[string]string `json:"follow_up_answers,omitempty"`
	RiskLevel       RiskLevel         `json:"risk_level"`
	Status          string            `json:"status"`
	GeneratedAt     time.Time         `json:"generated_at"`
	RuleVersion     string            `json:"rule_version"`
	Advice          Advice            `json:"advice"`
	Evidence        Evidence          `json:"evidence"`
	RuleSource      RuleSource        `json:"rule_source"`
	AIAnalysis      AIAnalysis        `json:"ai_analysis"`
}

type Advice struct {
	RiskLevel   RiskLevel `json:"risk_level"`
	ContactTeam bool      `json:"contact_team"`
	Urgency     string    `json:"urgency"`
	NextSteps   []string  `json:"next_steps"`
}

type Evidence struct {
	MatchedRuleID   string   `json:"matched_rule_id"`
	MatchedRuleName string   `json:"matched_rule_name"`
	MatchedKeywords []string `json:"matched_keywords"`
	Reason          string   `json:"reason"`
	AISummary       string   `json:"ai_summary,omitempty"`
	AISignals       []string `json:"ai_signals,omitempty"`
}

type RuleSource struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Source   string `json:"source"`
}

type AIAnalysis struct {
	Summary            string    `json:"summary"`
	Symptoms           []string  `json:"symptoms"`
	TemperatureCelsius *float64  `json:"temperature_celsius,omitempty"`
	Duration           string    `json:"duration,omitempty"`
	SeveritySignals    []string  `json:"severity_signals"`
	MissingFields      []string  `json:"missing_fields"`
	FollowUpQuestions  []string  `json:"follow_up_questions,omitempty"`
	UserExplanation    string    `json:"user_explanation,omitempty"`
	HandoffSummary     string    `json:"handoff_summary,omitempty"`
	SafetyWarnings     []string  `json:"safety_warnings,omitempty"`
	GeneratedBy        string    `json:"generated_by"`
	GeneratedAt        time.Time `json:"generated_at"`
}

type ContactRequest struct {
	ID             string    `json:"id"`
	AssessmentID   string    `json:"assessment_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"`
	Channel        string    `json:"channel"`
	Message        string    `json:"message,omitempty"`
	HandoffSummary string    `json:"handoff_summary"`
	CreatedAt      time.Time `json:"created_at"`
}

type ContactRequestInput struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
}

type EventLog struct {
	ID           string            `json:"id"`
	AssessmentID string            `json:"assessment_id,omitempty"`
	UserID       string            `json:"user_id,omitempty"`
	EventType    string            `json:"event_type"`
	CreatedAt    time.Time         `json:"created_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type EventInput struct {
	AssessmentID string            `json:"assessment_id,omitempty"`
	UserID       string            `json:"user_id,omitempty"`
	EventType    string            `json:"event_type"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type RuleImprovementSuggestion struct {
	ID                   string    `json:"id"`
	GeneratedAt          time.Time `json:"generated_at"`
	RuleVersion          string    `json:"rule_version"`
	Observation          string    `json:"observation"`
	Suggestion           string    `json:"suggestion"`
	RequiresHumanReview  bool      `json:"requires_human_review"`
	SupportingEventCount int       `json:"supporting_event_count"`
}

func NewID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf[:]))
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
