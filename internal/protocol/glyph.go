package protocol

import "fmt"

// Glyph is a 7-bit value (0..127) encoding base, rotation, and marker.
type Glyph uint8

// Field types.
type Base uint8
type Rotation uint8
type Marker uint8

const (
	BaseTriangle Base = iota
	BaseChevron
	BaseSquare
	BaseDiamond
)

const (
	RotationUp Rotation = iota
	RotationRight
	RotationDown
	RotationLeft
)

const (
	MarkerNone Marker = iota
	MarkerCenter
	MarkerTop
	MarkerRight
	MarkerBottom
	MarkerLeft
	// 6 and 7 are reserved/invalid
)

// EncodeGlyph packs base, rotation, and marker into a 7-bit glyph.
// Panics if any field is out of range; we pre-validate before calling.
func EncodeGlyph(base Base, rot Rotation, marker Marker) Glyph {
	if base > BaseDiamond || rot > RotationLeft || marker > MarkerLeft {
		panic("invalid glyph field")
	}
	return Glyph((uint8(base) << 5) | (uint8(rot) << 3) | uint8(marker))
}

// DecodeGlyph extracts the three fields from a glyph byte.
func DecodeGlyph(g Glyph) (Base, Rotation, Marker) {
	base := Base((g >> 5) & 0b11)
	rot := Rotation((g >> 3) & 0b11)
	marker := Marker(g & 0b111)
	return base, rot, marker
}

// IsValidVisualGlyph returns true only for field combinations that are visually
// unambiguous and defined in the specification.
func IsValidVisualGlyph(g Glyph) bool {
	base, rot, marker := DecodeGlyph(g)
	// Marker 6 and 7 are invalid for all bases.
	if marker > MarkerLeft {
		return false
	}
	// Base must be one of the four.
	if base > BaseDiamond {
		return false
	}
	// Rotation must be valid.
	if rot > RotationLeft {
		return false
	}
	// Square and Diamond only allow RotationUp.
	if (base == BaseSquare || base == BaseDiamond) && rot != RotationUp {
		return false
	}
	// All other combinations are valid.
	return true
}

// GlyphLabel returns a human-readable description for display/debug.
// It does NOT affect authorization.
func GlyphLabel(g Glyph) string {
	if !IsValidVisualGlyph(g) {
		return "INVALID"
	}
	base, rot, marker := DecodeGlyph(g)
	baseNames := []string{"Triangle", "Chevron", "Square", "Diamond"}
	rotNames := []string{"Up", "Right", "Down", "Left"}
	markerNames := []string{"None", "Center", "Top", "Right", "Bottom", "Left"}
	return fmt.Sprintf("%s/%s/%s", baseNames[base], rotNames[rot], markerNames[marker])
}
