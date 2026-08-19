package humanwhocodesobjectschema

import "errors"

// ErrSchemaDefinitionsMissing is thrown by NewObjectSchema with no definitions.
var ErrSchemaDefinitionsMissing = errors.New("Schema definitions missing.")

// UnExpectedKeyError mirrors esutils object-schema's UnexpectedKeyError.
type UnexpectedKeyError struct{ Key string }

func (e *UnexpectedKeyError) Error() string {
	return `Unexpected key "` + e.Key + `" found.`
}

// MissingKeyError mirrors MissingKeyError.
type MissingKeyError struct{ Key string }

func (e *MissingKeyError) Error() string {
	return `Missing required key "` + e.Key + `".`
}

// MissingDependentKeysError mirrors MissingDependentKeysError.
type MissingDependentKeysError struct {
	Key  string
	Keys []string
}

func (e *MissingDependentKeysError) Error() string {
	var joined string
	for i, k := range e.Keys {
		if i > 0 {
			joined += `, `
		}
		joined += `"` + k + `"`
	}
	return `Key "` + e.Key + `" requires keys ` + joined + "."
}

// WrapperError wraps an error from a merge/validate under a key, matching the
// JS message `Key "k": <source message>`.
type WrapperError struct {
	Key    string
	Source error
}

func (e *WrapperError) Error() string {
	return `Key "` + e.Key + `": ` + e.Source.Error()
}
