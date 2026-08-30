package approval_test

import (
	"testing"
	"time"

	"glyph-protocol/internal/approval"
	"glyph-protocol/internal/protocol"
)

func TestCreateAndConsumeApproval(t *testing.T) {
	store := approval.NewStore()
	now := time.Now()
	var hash [32]byte
	hash[0] = 0xab
	rec, err := store.CreateApproval("sess1", "req1", protocol.ActionSaveDraft, 2, 1, hash, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ConsumeApproval(rec.ID, "sess1", "req1", protocol.ActionSaveDraft, 2, 1, hash, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != rec.ID {
		t.Fatal("id mismatch")
	}
	// Second consume must fail
	_, err = store.ConsumeApproval(rec.ID, "sess1", "req1", protocol.ActionSaveDraft, 2, 1, hash, 1, now)
	if err == nil {
		t.Fatal("expected already used error")
	}
}

func TestApprovalMismatch(t *testing.T) {
	store := approval.NewStore()
	now := time.Now()
	var hash [32]byte
	rec, _ := store.CreateApproval("sess1", "req1", protocol.ActionSaveDraft, 2, 1, hash, 1, now)
	_, err := store.ConsumeApproval(rec.ID, "wrong-sess", "req1", protocol.ActionSaveDraft, 2, 1, hash, 1, now)
	if err == nil {
		t.Fatal("expected session mismatch")
	}
}
