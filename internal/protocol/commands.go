package protocol

import (
	"crypto/sha256"
	"encoding/hex"
)

// ActionID defines the fixed set of actions.
type ActionID uint8

const (
	ActionInvalid ActionID = iota
	ActionDisplaySlot
	ActionClassifySlot
	ActionSummarizeSlot
	ActionSaveDraft
)

// Permission is a bitmask.
type Permission uint32

const (
	PermNone Permission = 0
	PermView Permission = 1 << iota
	PermWriteDraft
)

// ActionSpec describes the properties of an action.
type ActionSpec struct {
	ID                   ActionID
	RequiresStagedSlot   bool
	RequiresConfirmation bool
	Permission           Permission
}

// CommandKey is a 21-bit value packed from three 7-bit glyphs.
type CommandKey uint32

// MakeCommandKey packs verb, object, and modifier into a uint32.
func MakeCommandKey(verb, object, modifier uint8) CommandKey {
	return CommandKey(uint32(verb)<<14 | uint32(object)<<7 | uint32(modifier))
}

// commandTable maps valid glyph triplets to their action spec.
// In V1 we use fixed slot constants for object/modifier.
// Slot values 0..7 represent runtime-owned slots.
var commandTable = map[CommandKey]ActionSpec{
	// DISPLAY_SLOT: verb=10, object=slot (0..7), modifier=0
	MakeCommandKey(10, 0, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 1, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 2, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 3, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 4, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 5, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 6, 0): {ActionDisplaySlot, true, false, PermView},
	MakeCommandKey(10, 7, 0): {ActionDisplaySlot, true, false, PermView},

	// CLASSIFY_SLOT: verb=20
	MakeCommandKey(20, 0, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 1, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 2, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 3, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 4, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 5, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 6, 0): {ActionClassifySlot, true, false, PermView},
	MakeCommandKey(20, 7, 0): {ActionClassifySlot, true, false, PermView},

	// SUMMARIZE_SLOT: verb=30
	MakeCommandKey(30, 0, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 1, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 2, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 3, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 4, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 5, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 6, 0): {ActionSummarizeSlot, true, false, PermView},
	MakeCommandKey(30, 7, 0): {ActionSummarizeSlot, true, false, PermView},

	// SAVE_DRAFT: verb=40, object=slot, modifier=0
	MakeCommandKey(40, 0, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 1, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 2, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 3, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 4, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 5, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 6, 0): {ActionSaveDraft, true, true, PermWriteDraft},
	MakeCommandKey(40, 7, 0): {ActionSaveDraft, true, true, PermWriteDraft},
}

// LookupCommand returns the action spec for a command key, or nil if not found.
func LookupCommand(key CommandKey) *ActionSpec {
	if spec, ok := commandTable[key]; ok {
		return &spec
	}
	return nil
}

// SlotIndexFromGlyph returns the slot index (0..7) if the glyph is a valid
// object slot number (0..7), otherwise -1.
// In V1 we treat the numeric value of the object glyph as the slot index.
func SlotIndexFromGlyph(g uint8) int {
	if g <= 7 {
		return int(g)
	}
	return -1
}

// HashContent computes SHA256 of a byte slice.
func HashContent(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// HexHash returns a hex string for a hash.
func HexHash(h [32]byte) string {
	return hex.EncodeToString(h[:])
}
