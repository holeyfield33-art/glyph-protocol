package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"glyph-protocol/internal/protocol"
)

type Outcome string

const (
	OutcomeSuccess              Outcome = "SUCCESS"
	OutcomeDenied               Outcome = "DENIED"
	OutcomeInvalidGlyph         Outcome = "INVALID_GLYPH"
	OutcomeInvalidStructure     Outcome = "INVALID_STRUCTURE"
	OutcomeInvalidJSON          Outcome = "INVALID_JSON"
	OutcomeUnknownCommand       Outcome = "UNKNOWN_COMMAND"
	OutcomeUnauthorized         Outcome = "UNAUTHORIZED"
	OutcomeRateLimited          Outcome = "RATE_LIMITED"
	OutcomeConfirmationRequired Outcome = "CONFIRMATION_REQUIRED"
	OutcomeApprovalExpired      Outcome = "APPROVAL_EXPIRED"
	OutcomeApprovalMismatch     Outcome = "APPROVAL_MISMATCH"
)

type Receipt struct {
	RequestID     string
	SessionIDHash string // SHA256 of session ID
	At            time.Time
	ProtocolV     uint32
	SequenceHash  [32]byte
	Action        protocol.ActionID
	SlotIndex     *uint8
	SlotVersion   *uint64
	ContentSHA256 *[32]byte
	Outcome       Outcome
	ReasonCode    string
}

// NewReceipt creates a receipt with the given data.
// sessionID is hashed before storage.
func NewReceipt(requestID, sessionID string, now time.Time, protocolV uint32, seq []int) Receipt {
	// Hash session ID.
	hash := sha256.Sum256([]byte(sessionID))
	sessionHash := hex.EncodeToString(hash[:])
	// Hash the sequence.
	seqBytes := make([]byte, len(seq)*2)
	for i, v := range seq {
		seqBytes[i*2] = byte(v >> 8)
		seqBytes[i*2+1] = byte(v & 0xff)
	}
	seqHash := sha256.Sum256(seqBytes)
	return Receipt{
		RequestID:     requestID,
		SessionIDHash: sessionHash,
		At:            now,
		ProtocolV:     protocolV,
		SequenceHash:  seqHash,
		Outcome:       OutcomeDenied, // default; caller overrides
	}
}
