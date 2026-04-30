package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"breast-cancer-side-effect-agent/internal/ai"
	"breast-cancer-side-effect-agent/internal/domain"
	"breast-cancer-side-effect-agent/internal/rules"
	"breast-cancer-side-effect-agent/internal/store"
)

type Server struct {
	store     *store.FileStore
	analyzer  *ai.Analyzer
	engine    *rules.Engine
	staticDir string
}

func NewServer(store *store.FileStore, analyzer *ai.Analyzer, engine *rules.Engine, staticDir string) *Server {
	return &Server{
		store:     store,
		analyzer:  analyzer,
		engine:    engine,
		staticDir: staticDir,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/result", s.handleResultPage)
	mux.HandleFunc("/history", s.handleHistoryPage)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.staticDir))))

	mux.HandleFunc("/api/healthz", s.handleHealth)
	mux.HandleFunc("/api/assessments", s.handleAssessments)
	mux.HandleFunc("/api/assessments/", s.handleAssessmentByID)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/rule-suggestions", s.handleRuleSuggestions)
	return loggingMiddleware(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
}

func (s *Server) handleResultPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.staticDir, "result.html"))
}

func (s *Server) handleHistoryPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.staticDir, "history.html"))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"ai_enabled":   s.analyzer.Enabled(),
		"rule_version": rules.Version,
		"now":          time.Now().UTC(),
	})
}

func (s *Server) handleAssessments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req domain.AssessmentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Description = strings.TrimSpace(req.Description)
	if req.UserID == "" {
		req.UserID = "demo-user"
	}
	if req.Description == "" {
		errorJSON(w, http.StatusBadRequest, "description is required")
		return
	}

	assessmentID := domain.NewID("asm")
	now := time.Now().UTC()
	_ = s.store.SaveEvent(newEvent(assessmentID, req.UserID, domain.EventAIAnalysisStarted, nil))
	analysis := s.analyzer.Analyze(r.Context(), req)
	_ = s.store.SaveEvent(newEvent(assessmentID, req.UserID, domain.EventAIAnalysisCompleted, map[string]string{
		"generated_by": analysis.GeneratedBy,
	}))

	advice, evidence, source := s.engine.Evaluate(req.Description, analysis)
	assessment := domain.Assessment{
		ID:              assessmentID,
		UserID:          req.UserID,
		Description:     req.Description,
		FollowUpAnswers: req.FollowUpAnswers,
		RiskLevel:       advice.RiskLevel,
		Status:          domain.AssessmentStatusOpen,
		GeneratedAt:     now,
		RuleVersion:     rules.Version,
		Advice:          advice,
		Evidence:        evidence,
		RuleSource:      source,
		AIAnalysis:      analysis,
	}
	explanation, warnings := s.analyzer.GenerateUserExplanation(r.Context(), assessment)
	assessment.AIAnalysis.UserExplanation = explanation
	assessment.AIAnalysis.SafetyWarnings = append(assessment.AIAnalysis.SafetyWarnings, warnings...)

	if err := s.store.SaveAssessment(assessment); err != nil {
		errorJSON(w, http.StatusInternalServerError, "save assessment failed")
		return
	}
	_ = s.store.SaveEvent(newEvent(assessment.ID, assessment.UserID, domain.EventAssessmentSubmitted, map[string]string{
		"risk_level": string(assessment.RiskLevel),
		"rule_id":    assessment.Evidence.MatchedRuleID,
	}))
	if len(assessment.AIAnalysis.FollowUpQuestions) > 0 {
		_ = s.store.SaveEvent(newEvent(assessment.ID, assessment.UserID, domain.EventFollowUpGenerated, map[string]string{
			"question_count": strconv.Itoa(len(assessment.AIAnalysis.FollowUpQuestions)),
		}))
	}

	needsFollowUp := len(assessment.AIAnalysis.FollowUpQuestions) > 0 &&
		len(req.FollowUpAnswers) == 0 &&
		assessment.RiskLevel != domain.RiskHigh

	writeJSON(w, http.StatusCreated, domain.AssessmentResponse{
		Assessment:        assessment,
		NeedsFollowUp:     needsFollowUp,
		FollowUpQuestions: assessment.AIAnalysis.FollowUpQuestions,
	})
}

func (s *Server) handleAssessmentByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/assessments/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		errorJSON(w, http.StatusNotFound, "assessment not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleGetAssessment(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "contact-requests" {
		s.handleCreateContactRequest(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "close" {
		s.handleCloseAssessment(w, r, id)
		return
	}
	errorJSON(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleGetAssessment(w http.ResponseWriter, r *http.Request, id string) {
	assessment, err := s.store.GetAssessment(id)
	if errors.Is(err, store.ErrNotFound) {
		errorJSON(w, http.StatusNotFound, "assessment not found")
		return
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "load assessment failed")
		return
	}
	writeJSON(w, http.StatusOK, assessment)
}

func (s *Server) handleCreateContactRequest(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	assessment, err := s.store.GetAssessment(id)
	if errors.Is(err, store.ErrNotFound) {
		errorJSON(w, http.StatusNotFound, "assessment not found")
		return
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "load assessment failed")
		return
	}
	var input domain.ContactRequestInput
	if err := decodeJSON(w, r, &input); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.Channel) == "" {
		input.Channel = "care_team"
	}
	handoff, warnings := s.analyzer.GenerateHandoffSummary(r.Context(), assessment)
	assessment.AIAnalysis.HandoffSummary = handoff
	assessment.AIAnalysis.SafetyWarnings = append(assessment.AIAnalysis.SafetyWarnings, warnings...)
	if err := s.store.SaveAssessment(assessment); err != nil {
		errorJSON(w, http.StatusInternalServerError, "update assessment failed")
		return
	}
	request := domain.ContactRequest{
		ID:             domain.NewID("ctr"),
		AssessmentID:   assessment.ID,
		UserID:         assessment.UserID,
		Status:         "open",
		Channel:        input.Channel,
		Message:        input.Message,
		HandoffSummary: handoff,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.SaveContactRequest(request); err != nil {
		errorJSON(w, http.StatusInternalServerError, "save contact request failed")
		return
	}
	_ = s.store.SaveEvent(newEvent(assessment.ID, assessment.UserID, domain.EventContactTeamClicked, map[string]string{"channel": input.Channel}))
	_ = s.store.SaveEvent(newEvent(assessment.ID, assessment.UserID, domain.EventHandoffSummaryGenerated, nil))
	writeJSON(w, http.StatusCreated, map[string]domain.ContactRequest{"contact_request": request})
}

func (s *Server) handleCloseAssessment(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	assessment, err := s.store.CloseAssessment(id, time.Now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		errorJSON(w, http.StatusNotFound, "assessment not found")
		return
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "close assessment failed")
		return
	}
	_ = s.store.SaveEvent(newEvent(assessment.ID, assessment.UserID, domain.EventAssessmentClosed, nil))
	writeJSON(w, http.StatusOK, assessment)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	writeJSON(w, http.StatusOK, map[string][]domain.Assessment{
		"assessments": s.store.ListAssessments(userID),
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var input domain.EventInput
	if err := decodeJSON(w, r, &input); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.EventType) == "" {
		errorJSON(w, http.StatusBadRequest, "event_type is required")
		return
	}
	event := newEvent(input.AssessmentID, input.UserID, input.EventType, input.Metadata)
	if err := s.store.SaveEvent(event); err != nil {
		errorJSON(w, http.StatusInternalServerError, "save event failed")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) handleRuleSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot := s.store.Snapshot()
	suggestion := s.analyzer.GenerateRuleImprovementSuggestion(snapshot.Assessments, snapshot.EventLogs, rules.Version)
	if err := s.store.SaveRuleImprovementSuggestion(suggestion); err != nil {
		errorJSON(w, http.StatusInternalServerError, "save suggestion failed")
		return
	}
	writeJSON(w, http.StatusCreated, suggestion)
}

func newEvent(assessmentID string, userID string, eventType string, metadata map[string]string) domain.EventLog {
	return domain.EventLog{
		ID:           domain.NewID("evt"),
		AssessmentID: assessmentID,
		UserID:       userID,
		EventType:    eventType,
		CreatedAt:    time.Now().UTC(),
		Metadata:     metadata,
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
