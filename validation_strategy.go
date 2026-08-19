package humanwhocodesobjectschema

// ValidationStrategy hosts the built-in validation strategies (the JS
// ValidationStrategy static class), plus a name -> strategy table so string
// strategies in a schema resolve identically.
type ValidationStrategy struct{}

type validateFunc func(v any) error

type validationStrategyTable struct{ m map[string]validateFunc }

// ValidationStrategies exposes the built-in strategies by name.
var ValidationStrategies = &validationStrategyTable{
	m: map[string]validateFunc{
		"array":   func(v any) error { return checkArray(v) },
		"boolean": func(v any) error { return checkBoolean(v) },
		"number":  func(v any) error { return checkNumber(v) },
		"object":  func(v any) error { return checkObject(v) },
		"object?": func(v any) error { return checkObjectOrNull(v) },
		"string":  func(v any) error { return checkString(v) },
		"string!": func(v any) error { return checkNonNullString(v) },
	},
}

func (t *validationStrategyTable) get(name string) (validateFunc, bool) {
	f, ok := t.m[name]
	return f, ok
}

// One-off error sentinels holding a fixed message (TypeError text from JS).
func typeErr(msg string) error {
	return &staticError{msg: msg}
}

type staticError struct{ msg string }

func (e *staticError) Error() string { return e.msg }

func checkArray(v any) error {
	if _, ok := v.([]any); !ok {
		return typeErr("Expected an array.")
	}
	return nil
}

func checkBoolean(v any) error {
	if _, ok := v.(bool); !ok {
		return typeErr("Expected a Boolean.")
	}
	return nil
}

func isNumber(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	}
	return false
}

func checkNumber(v any) error {
	if !isNumber(v) {
		return typeErr("Expected a number.")
	}
	return nil
}

func checkObject(v any) error {
	// JS: if (!value || typeof value !== "object") throw -> null fails, non-object fails.
	if v == nil || !isLikeObject(v) {
		return typeErr("Expected an object.")
	}
	return nil
}

func checkObjectOrNull(v any) error {
	// JS: typeof !== "object" -> null passes (typeof null === "object").
	if !isLikeObject(v) {
		return typeErr("Expected an object or null.")
	}
	return nil
}

func checkString(v any) error {
	if _, ok := v.(string); !ok {
		return typeErr("Expected a string.")
	}
	return nil
}

func checkNonNullString(v any) error {
	s, ok := v.(string)
	if !ok || len(s) == 0 {
		return typeErr("Expected a non-empty string.")
	}
	return nil
}
