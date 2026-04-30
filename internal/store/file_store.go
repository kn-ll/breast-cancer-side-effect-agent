package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"breast-cancer-side-effect-agent/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Snapshot struct {
	Assessments                []domain.Assessment                `json:"assessments"`
	ContactRequests            []domain.ContactRequest            `json:"contact_requests"`
	EventLogs                  []domain.EventLog                  `json:"event_logs"`
	RuleImprovementSuggestions []domain.RuleImprovementSuggestion `json:"rule_improvement_suggestions"`
}

type FileStore struct {
	path string
	mu   sync.Mutex
	data Snapshot
}

func NewFileStore(path string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	store := &FileStore{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) SaveAssessment(assessment domain.Assessment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, item := range s.data.Assessments {
		if item.ID == assessment.ID {
			s.data.Assessments[idx] = assessment
			return s.persistLocked()
		}
	}
	s.data.Assessments = append(s.data.Assessments, assessment)
	return s.persistLocked()
}

func (s *FileStore) GetAssessment(id string) (domain.Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.data.Assessments {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.Assessment{}, ErrNotFound
}

func (s *FileStore) ListAssessments(userID string) []domain.Assessment {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []domain.Assessment
	for _, item := range s.data.Assessments {
		if userID == "" || item.UserID == userID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].GeneratedAt.After(out[j].GeneratedAt)
	})
	return out
}

func (s *FileStore) CloseAssessment(id string, closedAt time.Time) (domain.Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, item := range s.data.Assessments {
		if item.ID == id {
			item.Status = domain.AssessmentStatusClosed
			if item.AIAnalysis.GeneratedAt.IsZero() {
				item.AIAnalysis.GeneratedAt = closedAt
			}
			s.data.Assessments[idx] = item
			if err := s.persistLocked(); err != nil {
				return domain.Assessment{}, err
			}
			return item, nil
		}
	}
	return domain.Assessment{}, ErrNotFound
}

func (s *FileStore) SaveContactRequest(request domain.ContactRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.ContactRequests = append(s.data.ContactRequests, request)
	return s.persistLocked()
}

func (s *FileStore) ListContactRequests(assessmentID string) []domain.ContactRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []domain.ContactRequest
	for _, item := range s.data.ContactRequests {
		if assessmentID == "" || item.AssessmentID == assessmentID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *FileStore) SaveEvent(event domain.EventLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.EventLogs = append(s.data.EventLogs, event)
	return s.persistLocked()
}

func (s *FileStore) ListEvents() []domain.EventLog {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]domain.EventLog, len(s.data.EventLogs))
	copy(out, s.data.EventLogs)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *FileStore) SaveRuleImprovementSuggestion(item domain.RuleImprovementSuggestion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.RuleImprovementSuggestions = append(s.data.RuleImprovementSuggestions, item)
	return s.persistLocked()
}

func (s *FileStore) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := Snapshot{
		Assessments:                append([]domain.Assessment(nil), s.data.Assessments...),
		ContactRequests:            append([]domain.ContactRequest(nil), s.data.ContactRequests...),
		EventLogs:                  append([]domain.EventLog(nil), s.data.EventLogs...),
		RuleImprovementSuggestions: append([]domain.RuleImprovementSuggestion(nil), s.data.RuleImprovementSuggestions...),
	}
	return copied
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = Snapshot{
			Assessments:                []domain.Assessment{},
			ContactRequests:            []domain.ContactRequest{},
			EventLogs:                  []domain.EventLog{},
			RuleImprovementSuggestions: []domain.RuleImprovementSuggestion{},
		}
		return s.persistLocked()
	}
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	if s.data.Assessments == nil {
		s.data.Assessments = []domain.Assessment{}
	}
	if s.data.ContactRequests == nil {
		s.data.ContactRequests = []domain.ContactRequest{}
	}
	if s.data.EventLogs == nil {
		s.data.EventLogs = []domain.EventLog{}
	}
	if s.data.RuleImprovementSuggestions == nil {
		s.data.RuleImprovementSuggestions = []domain.RuleImprovementSuggestion{}
	}
	return nil
}

func (s *FileStore) persistLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(raw, '\n'), 0o644)
}
