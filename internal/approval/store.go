package approval

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"glyph-protocol/internal/protocol"
)

// PendingApproval binds all parameters needed for a user confirmation.
type PendingApproval struct {
	ID            string
	SessionID     string
	RequestID     string
	Action        protocol.ActionID
	SlotIndex     uint8
	SlotVersion   uint64
	ContentSHA256 [32]byte
	PolicyVersion uint32
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Used          bool
}

// Store holds pending approvals.
type Store struct {
	approvals map[string]*PendingApproval
	mu        sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		approvals: make(map[string]*PendingApproval),
	}
}

// CreateApproval generates a new pending approval with a random ID.
func (s *Store) CreateApproval(
	sessionID, requestID string,
	action protocol.ActionID,
	slotIndex uint8,
	slotVersion uint64,
	contentSHA256 [32]byte,
	policyVersion uint32,
	now time.Time,
) (*PendingApproval, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(b)
	approval := &PendingApproval{
		ID:            id,
		SessionID:     sessionID,
		RequestID:     requestID,
		Action:        action,
		SlotIndex:     slotIndex,
		SlotVersion:   slotVersion,
		ContentSHA256: contentSHA256,
		PolicyVersion: policyVersion,
		CreatedAt:     now,
		ExpiresAt:     now.Add(60 * time.Second),
		Used:          false,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals[id] = approval
	return approval, nil
}

// GetApproval retrieves a pending approval by ID.
// Returns nil if not found or expired/used (but caller must check).
func (s *Store) GetApproval(id string) *PendingApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.approvals[id]
}

// ConsumeApproval atomically marks the approval as used and returns its details.
// It verifies all bindings match the provided parameters.
// Returns error if any check fails, otherwise returns the approval.
func (s *Store) ConsumeApproval(
	id, sessionID, requestID string,
	action protocol.ActionID,
	slotIndex uint8,
	slotVersion uint64,
	contentSHA256 [32]byte,
	policyVersion uint32,
	now time.Time,
) (*PendingApproval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	approval, ok := s.approvals[id]
	if !ok {
		return nil, errors.New("approval not found")
	}
	if approval.Used {
		return nil, errors.New("approval already used")
	}
	if approval.SessionID != sessionID {
		return nil, errors.New("session mismatch")
	}
	if approval.RequestID != requestID {
		return nil, errors.New("request ID mismatch")
	}
	if approval.Action != action {
		return nil, errors.New("action mismatch")
	}
	if approval.SlotIndex != slotIndex {
		return nil, errors.New("slot index mismatch")
	}
	if approval.SlotVersion != slotVersion {
		return nil, errors.New("slot version mismatch")
	}
	if approval.ContentSHA256 != contentSHA256 {
		return nil, errors.New("content hash mismatch")
	}
	if approval.PolicyVersion != policyVersion {
		return nil, errors.New("policy version mismatch")
	}
	if now.After(approval.ExpiresAt) {
		return nil, errors.New("approval expired")
	}
	// Mark used.
	approval.Used = true
	return approval, nil
}
