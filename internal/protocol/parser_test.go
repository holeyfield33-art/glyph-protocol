package protocol_test

import (
	"testing"

	"glyph-protocol/internal/protocol"
)

func TestParseStrictRequest_Valid(t *testing.T) {
	valid := []byte(`{"v":1,"seq":[10,3,0]}`)
	req, err := protocol.ParseStrictRequest(valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Version != 1 || len(req.Seq) != 3 {
		t.Fatalf("wrong parsed data: %+v", req)
	}
	if req.Seq[0] != 10 || req.Seq[1] != 3 || req.Seq[2] != 0 {
		t.Fatalf("wrong seq values: %v", req.Seq)
	}
}

func TestParseStrictRequest_DuplicateKey(t *testing.T) {
	dup := []byte(`{"v":1,"v":2,"seq":[10,3,0]}`)
	_, err := protocol.ParseStrictRequest(dup)
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestParseStrictRequest_OutOfRange(t *testing.T) {
	bad := []byte(`{"v":1,"seq":[128,3,0]}`)
	_, err := protocol.ParseStrictRequest(bad)
	if err == nil {
		t.Fatal("expected out of range error")
	}
}

func TestParseStrictRequest_TooShort(t *testing.T) {
	short := []byte(`{"v":1,"seq":[10,3]}`)
	_, err := protocol.ParseStrictRequest(short)
	if err == nil {
		t.Fatal("expected too short error")
	}
}

func TestParseStrictRequest_NotDivisible(t *testing.T) {
	bad := []byte(`{"v":1,"seq":[10,3,0,20]}`)
	_, err := protocol.ParseStrictRequest(bad)
	if err == nil {
		t.Fatal("expected not divisible error")
	}
}

func TestParseStrictRequest_UnknownKey(t *testing.T) {
	bad := []byte(`{"v":1,"seq":[10,3,0],"extra":true}`)
	_, err := protocol.ParseStrictRequest(bad)
	if err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestParseStrictRequest_WrongVersion(t *testing.T) {
	bad := []byte(`{"v":2,"seq":[10,3,0]}`)
	_, err := protocol.ParseStrictRequest(bad)
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestIsValidVisualGlyph(t *testing.T) {
	// Valid: triangle up none
	g := protocol.EncodeGlyph(protocol.BaseTriangle, protocol.RotationUp, protocol.MarkerNone)
	if !protocol.IsValidVisualGlyph(g) {
		t.Fatal("expected valid")
	}
	// Invalid marker
	if protocol.IsValidVisualGlyph(protocol.Glyph(0b00000110)) { // marker 6
		t.Fatal("expected invalid marker")
	}
	// Square with rotation != Up
	sq := protocol.EncodeGlyph(protocol.BaseSquare, protocol.RotationRight, protocol.MarkerNone)
	if protocol.IsValidVisualGlyph(sq) {
		t.Fatal("square must only allow RotationUp")
	}
}

func TestLookupCommand(t *testing.T) {
	key := protocol.MakeCommandKey(10, 3, 0)
	spec := protocol.LookupCommand(key)
	if spec == nil || spec.ID != protocol.ActionDisplaySlot {
		t.Fatal("expected DISPLAY_SLOT")
	}
	unknown := protocol.MakeCommandKey(99, 0, 0)
	if protocol.LookupCommand(unknown) != nil {
		t.Fatal("expected nil for unknown")
	}
}
