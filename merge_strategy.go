package humanwhocodesobjectschema

import "fmt"

// mergeFunc is a merge strategy over two values (mirrors the JS MergeStrategy
// statics). It returns an error so subschema merges can propagate up and be
// wrapped like JS exceptions.
type mergeFunc func(v1, v2 any) (any, error)

// mergeStrategies hosts the built-in merge strategies by name, mirroring the
// JS MergeStrategy static class used to resolve string strategies.
type mergeStrategies struct{ m map[string]mergeFunc }

// MergeStrategies exposes the built-in strategies by name.
var MergeStrategies = &mergeStrategies{
	m: map[string]mergeFunc{
		"overwrite": func(v1, v2 any) (any, error) { return v2, nil },
		"replace": func(v1, v2 any) (any, error) {
			if v2 != nil {
				return v2, nil
			}
			return v1, nil
		},
		"assign": func(v1, v2 any) (any, error) { return assign(v1, v2), nil },
	},
}

func (t *mergeStrategies) get(name string) (mergeFunc, bool) {
	f, ok := t.m[name]
	return f, ok
}

// assign mirrors Object.assign({}, v1, v2): a shallow copy of v1 then v2's
// own enumerable keys. JS Object.assign also copies string characters and
// array elements by index, so those are handled too.
func assign(v1, v2 any) any {
	out := map[string]any{}
	copyProps := func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				out[k] = val
			}
		case map[string]string:
			for k, val := range t {
				out[k] = val
			}
		case []any:
			for i, val := range t {
				out[keyIdx(i)] = val
			}
		case string:
			for i, r := range []rune(t) {
				out[keyIdx(i)] = string(r)
			}
		}
	}
	copyProps(v1)
	copyProps(v2)
	return out
}

func keyIdx(i int) string {
	return fmt.Sprintf("%d", i)
}

// asObject coerces a value to a map[string]any ("object") when possible.
func asObject(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[string]string:
		out := map[string]any{}
		for k, s := range m {
			out[k] = s
		}
		return out, true
	}
	return nil, false
}

// asMap returns a value as a map[string]any, or nil.
func asMap(v any) map[string]any {
	m, _ := asObject(v)
	return m
}

// isLikeObject mirrors JS `typeof value === "object"` (null is object).
func isLikeObject(v any) bool {
	if v == nil {
		return true
	}
	switch v.(type) {
	case map[string]any, map[string]string, []any, []string:
		return true
	}
	return false
}
