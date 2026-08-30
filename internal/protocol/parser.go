package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	MaxBodyBytes    = 1024
	MaxGlyphs       = 36
	MinGlyphs       = 3
	MaxInstructions = 12 // 36/3
	InstructionsLen = 3
	ProtocolVersion = 1
)

// ParseStrictRequest reads the raw body, validates JSON shape, and returns a
// validated Request struct. Returns an error for any malformation.
func ParseStrictRequest(body []byte) (*Request, error) {
	if len(body) > MaxBodyBytes {
		return nil, errors.New("body exceeds max size")
	}
	if !json.Valid(body) {
		return nil, errors.New("invalid JSON")
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	// Expect object start.
	t, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil, errors.New("top-level must be an object")
	}

	seen := make(map[string]struct{})
	var req Request
	req.Seq = []int{} // ensure not nil

	// Iterate over key/value pairs.
	for dec.More() {
		// Read key.
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, errors.New("object key is not a string")
		}
		if _, exists := seen[key]; exists {
			return nil, DuplicateKeyError{Key: key}
		}
		seen[key] = struct{}{}

		// Decode value based on key.
		switch key {
		case "v":
			var v json.Number
			if err := dec.Decode(&v); err != nil {
				return nil, fmt.Errorf("invalid v: %w", err)
			}
			val, err := v.Int64()
			if err != nil {
				return nil, errors.New("v must be an integer")
			}
			if val != ProtocolVersion {
				return nil, fmt.Errorf("unsupported protocol version: %d", val)
			}
			req.Version = int(val)
		case "seq":
			// Decode seq as an array of JSON numbers.
			// We'll read tokens manually to enforce strict array and integer types.
			tok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("failed to parse seq: %w", err)
			}
			delim, ok := tok.(json.Delim)
			if !ok || delim != '[' {
				return nil, errors.New("seq must be an array")
			}
			// Now read numbers until closing bracket.
			var seq []int
			for {
				t, err := dec.Token()
				if err == io.EOF {
					return nil, errors.New("unexpected end of seq array")
				}
				if err != nil {
					return nil, fmt.Errorf("seq token error: %w", err)
				}
				if delim, ok := t.(json.Delim); ok && delim == ']' {
					break // end of array
				}
				num, ok := t.(json.Number)
				if !ok {
					return nil, errors.New("seq element must be a number")
				}
				val, err := num.Int64()
				if err != nil {
					return nil, fmt.Errorf("seq element not an integer: %s", num)
				}
				if val < 0 || val > 127 {
					return nil, fmt.Errorf("glyph value out of range: %d", val)
				}
				seq = append(seq, int(val))
			}
			req.Seq = seq
		default:
			// Unknown top-level key – reject.
			return nil, fmt.Errorf("unknown key: %s", key)
		}
	}

	// Ensure we reached the end of the object.
	t, err = dec.Token()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("trailing data after object: %w", err)
	}
	if delim, ok := t.(json.Delim); ok && delim != '}' {
		return nil, errors.New("expected end of object")
	}

	// Validate required keys.
	if _, ok := seen["v"]; !ok {
		return nil, errors.New("missing v")
	}
	if _, ok := seen["seq"]; !ok {
		return nil, errors.New("missing seq")
	}

	// Validate seq length constraints.
	if len(req.Seq) < MinGlyphs {
		return nil, fmt.Errorf("seq too short: %d < %d", len(req.Seq), MinGlyphs)
	}
	if len(req.Seq) > MaxGlyphs {
		return nil, fmt.Errorf("seq too long: %d > %d", len(req.Seq), MaxGlyphs)
	}
	if len(req.Seq)%InstructionsLen != 0 {
		return nil, fmt.Errorf("seq length %d not divisible by %d", len(req.Seq), InstructionsLen)
	}
	if len(req.Seq)/InstructionsLen > MaxInstructions {
		return nil, fmt.Errorf("too many instructions: %d > %d", len(req.Seq)/InstructionsLen, MaxInstructions)
	}

	return &req, nil
}
