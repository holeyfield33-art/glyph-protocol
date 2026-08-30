package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"glyph-protocol/internal/approval"
	"glyph-protocol/internal/engine"
	"glyph-protocol/internal/protocol"
	"glyph-protocol/internal/session"
)

var (
	sessionStore  = session.NewStore()
	approvalStore = approval.NewStore()
	eng           = engine.NewEngine(sessionStore, approvalStore)
)

func main() {
	// Static UI.
	http.Handle("/", http.FileServer(http.Dir("./internal/ui/static")))

	// API endpoints.
	http.HandleFunc("/v1/session", handleSession)
	http.HandleFunc("/v1/stage", handleStage)
	http.HandleFunc("/v1/propose", handlePropose)
	http.HandleFunc("/v1/approve", handleApprove)
	http.HandleFunc("/v1/drafts", handleDrafts)

	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}

// handleSession creates a new demo session with full permissions.
func handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, err := sessionStore.CreateSession(session.PermView | session.PermWriteDraft)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// For demo, we use Bearer token as session ID.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"session_id": sess.ID})
}

// handleStage stages user-entered text into a slot.
func handleStage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := extractSession(r)
	if sessionID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Slot int    `json:"slot"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Slot < 0 || req.Slot > 7 {
		http.Error(w, "slot must be 0..7", http.StatusBadRequest)
		return
	}
	content := []byte(req.Text)
	if len(content) > 500 {
		http.Error(w, "text too long (max 500 bytes)", http.StatusBadRequest)
		return
	}
	slot, err := sessionStore.StageUserText(sessionID, uint8(req.Slot), content, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"slot":       slot.SlotIndex,
		"version":    slot.Version,
		"sha256":     protocol.HexHash(slot.SHA256),
		"expires_at": slot.ExpiresAt.Format(time.RFC3339),
		"provenance": string(slot.Provenance),
	})
}

// handlePropose accepts a glyph protocol request.
func handlePropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := extractSession(r)
	if sessionID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
	if err != nil {
		http.Error(w, "body too large", http.StatusBadRequest)
		return
	}
	now := time.Now()
	result, err := eng.ProcessRequest(sessionID, body, now)
	if err != nil {
		// Log error as receipt? For simplicity, we return a denied response.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":      "DENIED",
			"reason_code": err.Error(),
		})
		return
	}
	// Build response.
	resp := map[string]interface{}{
		"request_id": generateRequestID(),
	}
	if result.PendingApproval != nil {
		resp["status"] = "CONFIRMATION_REQUIRED"
		resp["approval_id"] = result.PendingApproval.ApprovalID
		resp["expires_at"] = result.PendingApproval.ExpiresAt.Format(time.RFC3339)
	} else {
		resp["status"] = "SUCCESS"
		// Optionally include action results.
		actionResults := make([]map[string]interface{}, len(result.Actions))
		for i, a := range result.Actions {
			actionResults[i] = map[string]interface{}{
				"action": a.ActionID,
				"slot":   a.SlotIdx,
				"result": a.Result,
			}
		}
		resp["actions"] = actionResults
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleApprove approves or denies a pending approval.
func handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := extractSession(r)
	if sessionID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		ApprovalID string `json:"approval_id"`
		Decision   string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Decision != "approve" && req.Decision != "deny" {
		http.Error(w, "decision must be 'approve' or 'deny'", http.StatusBadRequest)
		return
	}
	// Retrieve the pending approval.
	approvalRec := approvalStore.GetApproval(req.ApprovalID)
	if approvalRec == nil {
		http.Error(w, "approval not found", http.StatusNotFound)
		return
	}
	now := time.Now()
	if req.Decision == "deny" {
		// Mark as used to prevent later use.
		approvalStore.ConsumeApproval(req.ApprovalID, sessionID, approvalRec.RequestID,
			approvalRec.Action, approvalRec.SlotIndex, approvalRec.SlotVersion,
			approvalRec.ContentSHA256, approvalRec.PolicyVersion, now)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "DENIED"})
		return
	}
	// Approve: atomically consume approval, mark slot used, and save draft.
	// We need to re-fetch the slot and verify its current state matches the approval.
	slot, err := sessionStore.GetUsableSlot(sessionID, approvalRec.SlotIndex, now, true)
	if err != nil {
		http.Error(w, "slot no longer valid: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Consume approval with all bindings.
	_, err = approvalStore.ConsumeApproval(req.ApprovalID, sessionID, approvalRec.RequestID,
		approvalRec.Action, slot.SlotIndex, slot.Version, slot.SHA256,
		approvalRec.PolicyVersion, now)
	if err != nil {
		http.Error(w, "approval validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Mark slot used (atomic with approval consumption).
	usedSlot, err := sessionStore.MarkSlotUsed(sessionID, approvalRec.SlotIndex, now)
	if err != nil {
		http.Error(w, "failed to mark slot used: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Save the draft.
	if err := sessionStore.AppendDraft(sessionID, usedSlot.Content); err != nil {
		http.Error(w, "failed to save draft: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "SUCCESS"})
}

// handleDrafts returns the session's drafts.
func handleDrafts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := extractSession(r)
	if sessionID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	drafts, err := sessionStore.GetDrafts(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Convert to strings for output.
	strDrafts := make([]string, len(drafts))
	for i, d := range drafts {
		strDrafts[i] = string(d)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"drafts": strDrafts,
	})
}

// Helper to extract session ID from Authorization header.
func extractSession(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// generateRequestID (simple random hex).
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// fallback
		return fmt.Sprintf("%d%x", time.Now().UnixNano(), b)
	}
	return hex.EncodeToString(b)
}
