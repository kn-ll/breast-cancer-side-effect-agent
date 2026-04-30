package store

import (
	"path/filepath"
	"testing"
	"time"

	"breast-cancer-side-effect-agent/internal/domain"
)

func TestFileStoreAssessmentRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	fs, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	assessment := domain.Assessment{
		ID:          "asm_test",
		UserID:      "user_1",
		Description: "发热 38.5",
		RiskLevel:   domain.RiskHigh,
		Status:      domain.AssessmentStatusOpen,
		GeneratedAt: time.Now().UTC(),
	}
	if err := fs.SaveAssessment(assessment); err != nil {
		t.Fatal(err)
	}
	got, err := fs.GetAssessment("asm_test")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "user_1" {
		t.Fatalf("user = %s, want user_1", got.UserID)
	}
	if len(fs.ListAssessments("user_1")) != 1 {
		t.Fatalf("expected one assessment")
	}
}

func TestFileStoreEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	fs, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.SaveEvent(domain.EventLog{ID: "evt_1", EventType: domain.EventResultViewed, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if len(fs.ListEvents()) != 1 {
		t.Fatalf("expected one event")
	}
}
