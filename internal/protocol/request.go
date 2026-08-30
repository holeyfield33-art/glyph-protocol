package protocol

// Request is the only accepted JSON schema.
type Request struct {
	Version int   `json:"v"`
	Seq     []int `json:"seq"` // raw ints; we validate bounds later.
}

// DuplicateKeyError is used by the strict parser.
type DuplicateKeyError struct {
	Key string
}

func (e DuplicateKeyError) Error() string {
	return "duplicate key: " + e.Key
}
