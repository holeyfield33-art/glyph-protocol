package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Provenance indicates the origin of the staged content.
type Provenance string

const (
	ProvenanceUserTyped    Provenance = "user_typed"
	ProvenanceLLMGenerated Provenance = "llm_generated" // not used in V1 for staging, but type exists
)

// StagedSlot holds immutable, versioned, session-owned content.
type StagedSlot struct {
	SlotIndex  uint8
	Version    uint64
	SessionID  string
	Content    []byte
	SHA256     [32]byte
	Provenance Provenance
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Used       bool // for one-time use (e.g., after save)
}

// Permission is a bitmask (redeclared for session package independence).
type Permission uint32

const (
	PermNone Permission = 0
	PermView Permission = 1 << iota
	PermWriteDraft
)

// Session holds all runtime state for a demo session.
type Session struct {
	ID          string
	Permissions Permission
	Slots       [8]*StagedSlot // index by slot number
	Drafts      [][]byte       // in-memory draft store (immutable copies)
	RequestLog  []time.Time    // for rate limiting
	mu          sync.RWMutex
}

// Store manages all sessions and slots.
type Store struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

// CreateSession generates a new session with the given permissions.
func (s *Store) CreateSession(perms Permission) (*Session, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(b)
	sess := &Session{
		ID:          id,
		Permissions: perms,
		Slots:       [8]*StagedSlot{},
		Drafts:      [][]byte{},
		RequestLog:  []time.Time{},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sess
	return sess, nil
}

// GetSession returns a session by ID, or nil if not found.
func (s *Store) GetSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

// StageUserText creates a new slot with user-typed content.
// It verifies size, slot index, and session existence.
func (s *Store) StageUserText(sessionID string, slotIndex uint8, content []byte, now time.Time) (*StagedSlot, error) {
	if slotIndex > 7 {
		return nil, errors.New("slot index out of range (0..7)")
	}
	if len(content) > 500 {
		return nil, errors.New("content exceeds 500 bytes")
	}
	sess := s.GetSession(sessionID)
	if sess == nil {
		return nil, errors.New("session not found")
	}
	// Copy content to avoid external mutation.
	copyContent := make([]byte, len(content))
	copy(copyContent, content)
	hash := sha256.Sum256(copyContent)

	slot := &StagedSlot{
		SlotIndex:  slotIndex,
		SessionID:  sessionID,
		Content:    copyContent,
		SHA256:     hash,
		Provenance: ProvenanceUserTyped,
		CreatedAt:  now,
		ExpiresAt:  now.Add(60 * time.Second),
		Used:       false,
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	// Increment version: if slot exists, new version = old version + 1
	if old := sess.Slots[slotIndex]; old != nil {
		slot.Version = old.Version + 1
	} else {
		slot.Version = 1
	}
	sess.Slots[slotIndex] = slot
	return slot, nil
}

// GetUsableSlot retrieves a slot for a given session and index, checking
// ownership, expiration, and optionally one-time use.
func (s *Store) GetUsableSlot(sessionID string, slotIndex uint8, now time.Time, requireUnused bool) (*StagedSlot, error) {
	if slotIndex > 7 {
		return nil, errors.New("invalid slot index")
	}
	sess := s.GetSession(sessionID)
	if sess == nil {
		return nil, errors.New("session not found")
	}
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	slot := sess.Slots[slotIndex]
	if slot == nil {
		return nil, errors.New("slot is empty")
	}
	if slot.SessionID != sessionID {
		return nil, errors.New("slot belongs to another session")
	}
	if now.After(slot.ExpiresAt) {
		return nil, errors.New("slot expired")
	}
	if requireUnused && slot.Used {
		return nil, errors.New("slot already used")
	}
	// Verify content integrity.
	recomputed := sha256.Sum256(slot.Content)
	if recomputed != slot.SHA256 {
		return nil, errors.New("content hash mismatch")
	}
	return slot, nil
}

// MarkSlotUsed marks the slot as used (one-time) after a successful save.
// It returns the slot if it was still usable and unmarked.
func (s *Store) MarkSlotUsed(sessionID string, slotIndex uint8, now time.Time) (*StagedSlot, error) {
	slot, err := s.GetUsableSlot(sessionID, slotIndex, now, true) // require unused
	if err != nil {
		return nil, err
	}
	sess := s.GetSession(sessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	// Double-check under lock that it hasn't been used concurrently.
	if sess.Slots[slotIndex].Used {
		return nil, errors.New("slot already used (concurrent)")
	}
	sess.Slots[slotIndex].Used = true
	return slot, nil
}

// AppendDraft appends an immutable copy of the content to the session's draft store.
func (s *Store) AppendDraft(sessionID string, content []byte) error {
	sess := s.GetSession(sessionID)
	if sess == nil {
		return errors.New("session not found")
	}
	// Copy before storing.
	copyContent := make([]byte, len(content))
	copy(copyContent, content)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.Drafts = append(sess.Drafts, copyContent)
	return nil
}

// GetDrafts returns a copy of the session's drafts.
func (s *Store) GetDrafts(sessionID string) ([][]byte, error) {
	sess := s.GetSession(sessionID)
	if sess == nil {
		return nil, errors.New("session not found")
	}
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	result := make([][]byte, len(sess.Drafts))
	for i, d := range sess.Drafts {
		c := make([]byte, len(d))
		copy(c, d)
		result[i] = c
	}
	return result, nil
}

// CheckRateLimit enforces 10 requests per 60 seconds per session.
// Returns true if within limit, false if rate limited.
func (s *Store) CheckRateLimit(sessionID string, now time.Time) bool {
	sess := s.GetSession(sessionID)
	if sess == nil {
		return false
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	// Remove timestamps older than 60 seconds.
	cutoff := now.Add(-60 * time.Second)
	kept := 0
	for _, t := range sess.RequestLog {
		if t.After(cutoff) {
			sess.RequestLog[kept] = t
			kept++
		}
	}
	sess.RequestLog = sess.RequestLog[:kept]
	if len(sess.RequestLog) >= 10 {
		return false
	}
	sess.RequestLog = append(sess.RequestLog, now)
	return true
}
