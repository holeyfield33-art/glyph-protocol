package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"glyph-protocol/internal/approval"
	"glyph-protocol/internal/protocol"
	"glyph-protocol/internal/session"
)

// Engine holds references to the session store and approval store.
type Engine struct {
	sessionStore  *session.Store
	approvalStore *approval.Store
	policyVersion uint32
}

func NewEngine(sessionStore *session.Store, approvalStore *approval.Store) *Engine {
	return &Engine{
		sessionStore:  sessionStore,
		approvalStore: approvalStore,
		policyVersion: 1,
	}
}

// ProcessRequest validates the raw request, executes actions (or creates approvals),
// and returns a result with a receipt.
func (e *Engine) ProcessRequest(sessionID string, rawBody []byte, now time.Time) (*EngineResult, error) {
	// 1. Parse strict request.
	req, err := protocol.ParseStrictRequest(rawBody)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 2. Check session existence and rate limit.
	sess := e.sessionStore.GetSession(sessionID)
	if sess == nil {
		return nil, errors.New("session not found")
	}
	if !e.sessionStore.CheckRateLimit(sessionID, now) {
		return nil, errors.New("rate limited")
	}

	// 3. Split seq into triplets and build command keys, also validate each glyph visually.
	instructions := len(req.Seq) / 3
	var commandKeys []protocol.CommandKey
	var slotIndices []int
	for i := 0; i < instructions; i++ {
		verb := uint8(req.Seq[i*3])
		obj := uint8(req.Seq[i*3+1])
		mod := uint8(req.Seq[i*3+2])
		// Validate each glyph visually.
		if !protocol.IsValidVisualGlyph(protocol.Glyph(verb)) ||
			!protocol.IsValidVisualGlyph(protocol.Glyph(obj)) ||
			!protocol.IsValidVisualGlyph(protocol.Glyph(mod)) {
			return nil, errors.New("invalid visual glyph in seq")
		}
		key := protocol.MakeCommandKey(verb, obj, mod)
		commandKeys = append(commandKeys, key)
		// Extract slot index from object glyph (only if command requires slot).
		slotIndices = append(slotIndices, protocol.SlotIndexFromGlyph(obj))
	}

	// 4. Resolve command specs for all instructions (all-or-nothing).
	var specs []protocol.ActionSpec
	for _, key := range commandKeys {
		spec := protocol.LookupCommand(key)
		if spec == nil {
			return nil, fmt.Errorf("unknown command key: %v", key)
		}
		specs = append(specs, *spec)
	}

	// 5. Validate all slots and authorizations upfront.
	for i, spec := range specs {
		if spec.RequiresStagedSlot {
			slotIdx := slotIndices[i]
			if slotIdx < 0 || slotIdx > 7 {
				return nil, errors.New("invalid slot index in command")
			}
			// Check slot exists, belongs to session, not expired.
			_, err := e.sessionStore.GetUsableSlot(sessionID, uint8(slotIdx), now, false)
			if err != nil {
				return nil, fmt.Errorf("slot validation failed: %w", err)
			}
		}
		// Check permission.
		if (session.Permission(sess.Permissions) & session.Permission(spec.Permission)) != session.Permission(spec.Permission) {
			return nil, fmt.Errorf("missing permission for action %v", spec.ID)
		}
	}

	// 6. Execute each instruction (all or nothing).
	var actionsExecuted []ActionExecuted
	var pendingApproval *PendingApprovalInfo

	for i, spec := range specs {
		slotIdx := slotIndices[i]
		var slot *session.StagedSlot
		if spec.RequiresStagedSlot {
			var err error
			slot, err = e.sessionStore.GetUsableSlot(sessionID, uint8(slotIdx), now, false)
			if err != nil {
				return nil, fmt.Errorf("slot retrieval failed: %w", err)
			}
		}
		// Perform action based on spec.ID.
		switch spec.ID {
		case protocol.ActionDisplaySlot:
			if slot == nil {
				return nil, errors.New("internal: display requires slot")
			}
			// Read-only: return text (limited to 500 bytes).
			text := string(slot.Content)
			if len(text) > 500 {
				text = text[:500]
			}
			actionsExecuted = append(actionsExecuted, ActionExecuted{
				ActionID: spec.ID,
				SlotIdx:  uint8(slotIdx),
				Result:   text,
			})
		case protocol.ActionClassifySlot:
			if slot == nil {
				return nil, errors.New("internal: classify requires slot")
			}
			// Deterministic classifier: classify by content.
			classification := classifyContent(slot.Content)
			actionsExecuted = append(actionsExecuted, ActionExecuted{
				ActionID: spec.ID,
				SlotIdx:  uint8(slotIdx),
				Result:   classification,
			})
		case protocol.ActionSummarizeSlot:
			if slot == nil {
				return nil, errors.New("internal: summarize requires slot")
			}
			// Summary: first 120 runes (UTF-8 safe).
			summary := summarizeContent(slot.Content)
			actionsExecuted = append(actionsExecuted, ActionExecuted{
				ActionID: spec.ID,
				SlotIdx:  uint8(slotIdx),
				Result:   summary,
			})
		case protocol.ActionSaveDraft:
			// This action requires confirmation and does not execute immediately.
			if slot == nil {
				return nil, errors.New("internal: save draft requires slot")
			}
			// Create a pending approval.
			reqID := generateRequestID()
			approvalRec, err := e.approvalStore.CreateApproval(
				sessionID,
				reqID,
				spec.ID,
				slot.SlotIndex,
				slot.Version,
				slot.SHA256,
				e.policyVersion,
				now,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create approval: %w", err)
			}
			pendingApproval = &PendingApprovalInfo{
				ApprovalID: approvalRec.ID,
				RequestID:  reqID,
				ExpiresAt:  approvalRec.ExpiresAt,
			}
			// We do not execute the save; we'll handle it via the approve endpoint.
		default:
			return nil, fmt.Errorf("unhandled action ID: %v", spec.ID)
		}
	}

	// Return result.
	return &EngineResult{
		Success:         true,
		Actions:         actionsExecuted,
		PendingApproval: pendingApproval,
	}, nil
}

// ActionExecuted holds the result of a read-only action.
type ActionExecuted struct {
	ActionID protocol.ActionID
	SlotIdx  uint8
	Result   string // string representation for display
}

// PendingApprovalInfo returned when SAVE_DRAFT is requested.
type PendingApprovalInfo struct {
	ApprovalID string
	RequestID  string
	ExpiresAt  time.Time
}

// EngineResult is the outcome of processing a request.
type EngineResult struct {
	Success         bool
	Actions         []ActionExecuted
	PendingApproval *PendingApprovalInfo
}

// Helper functions.

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// fallback to timestamp + random suffix
		return fmt.Sprintf("%d%x", time.Now().UnixNano(), b)
	}
	return hex.EncodeToString(b)
}

func classifyContent(content []byte) string {
	if len(content) == 0 {
		return "EMPTY"
	}
	if len(content) < 100 {
		return "SHORT_TEXT"
	}
	if len(content) >= 100 && len(content) < 400 {
		return "LONG_TEXT"
	}
	// Check for digits.
	for _, b := range content {
		if b >= '0' && b <= '9' {
			return "CONTAINS_DIGITS"
		}
	}
	return "LONG_TEXT"
}

func summarizeContent(content []byte) string {
	// Take first 120 Unicode code points (runes).
	runes := []rune(string(content))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120]) + "..."
}
