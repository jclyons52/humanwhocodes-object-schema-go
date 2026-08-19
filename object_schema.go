package humanwhocodesobjectschema

import (
	"fmt"
	"sort"
)

// ObjectSchema mirrors the JS ObjectSchema: a schema describing how to merge
// and validate object keys. Definitions may reference built-in strategies by
// string name ("overwrite", "assign", "array", "object"...), nest subschemas
// via "schema", and mark keys "required" / "requires".
type ObjectSchema struct {
	strategies   map[string]*definition
	requiredKeys map[string]*definition
}

// definition is one key's scheme after normalization.
type definition struct {
	required bool
	requires []string
	merge    mergeFunc
	validate validateFunc
}

// validateDefinition mirrors the private validateDefinition() of the JS source.
func validateDefinition(name string, strategy map[string]any) error {
	hasSchema := false
	if schema, ok := strategy["schema"]; ok && schema != nil {
		if isLikeObject(schema) && !isScalar(schema) {
			hasSchema = true
		} else {
			return typeErr("Schema must be an object.")
		}
	}
	if merge, ok := strategy["merge"].(string); ok {
		if _, found := MergeStrategies.get(merge); !found {
			return typeErr(fmt.Sprintf(`Definition for key "%s" missing valid merge strategy.`, name))
		}
	} else if !hasSchema {
		if _, isFn := strategy["merge"].(mergeFunc); !isFn && strategy["merge"] == nil {
			return typeErr(fmt.Sprintf(`Definition for key "%s" must have a merge property.`, name))
		}
	}
	if validate, ok := strategy["validate"].(string); ok {
		if _, found := ValidationStrategies.get(validate); !found {
			return typeErr(fmt.Sprintf(`Definition for key "%s" missing valid validation strategy.`, name))
		}
	} else if !hasSchema {
		if strategy["validate"] == nil {
			return typeErr(fmt.Sprintf(`Definition for key "%s" must have a validate() method.`, name))
		}
	}
	return nil
}

// NewObjectSchema builds a schema from a definitions map, mirroring the JS
// ObjectSchema constructor (it throws on invalid definitions).
func NewObjectSchema(definitions map[string]any) (*ObjectSchema, error) {
	if definitions == nil {
		return nil, ErrSchemaDefinitionsMissing
	}
	os := &ObjectSchema{
		strategies:   map[string]*definition{},
		requiredKeys: map[string]*definition{},
	}
	keys := sortedKeys(definitions)
	for _, key := range keys {
		dv, ok := definitions[key].(map[string]any)
		if !ok {
			dv = map[string]any{}
		}
		if err := validateDefinition(key, dv); err != nil {
			return nil, err
		}
		def := &definition{}
		if req, ok := dv["required"].(bool); ok {
			def.required = req
		}
		if reqs, ok := dv["requires"].([]any); ok {
			for _, r := range reqs {
				if s, ok := r.(string); ok {
					def.requires = append(def.requires, s)
				}
			}
		}
		// normalize subschema merge/validate (wins over string strategies,
		// matching JS where the schema rewrite replaces them with functions)
		if schema, ok := dv["schema"].(map[string]any); ok && schema != nil {
			sub, err := NewObjectSchema(schema)
			if err != nil {
				return nil, err
			}
			def.merge = func(v1, v2 any) (any, error) {
				return sub.Merge(v1, v2)
			}
			def.validate = func(v any) error {
				objFn, _ := ValidationStrategies.get("object")
				if err := objFn(v); err != nil {
					return err
				}
				return sub.Validate(asMap(v))
			}
		} else {
			// normalize string merge
			if merge, ok := dv["merge"].(string); ok {
				f, _ := MergeStrategies.get(merge)
				def.merge = f
			}
			// normalize string validate
			if validate, ok := dv["validate"].(string); ok {
				f, _ := ValidationStrategies.get(validate)
				def.validate = f
			}
		}
		os.strategies[key] = def
		if def.required {
			os.requiredKeys[key] = def
		}
	}
	return os, nil
}

// HasKey reports whether a strategy is registered for the object key.
func (o *ObjectSchema) HasKey(key string) bool {
	_, ok := o.strategies[key]
	return ok
}

// Merge merges objects into a new object using each key's merge strategy.
// Mirrors JS ObjectSchema.merge(...objects).
func (o *ObjectSchema) Merge(objects ...any) (map[string]any, error) {
	if len(objects) < 2 {
		return nil, typeErr("merge() requires at least two arguments.")
	}
	for _, obj := range objects {
		if obj == nil || !isLikeObject(obj) {
			return nil, typeErr("All arguments must be objects.")
		}
	}
	result := map[string]any{}
	for _, obj := range objects {
		om, _ := asObject(obj)
		if err := o.Validate(om); err != nil {
			return nil, err
		}
		for _, key := range sortedMapKeys(resultKeys(result, om)) {
			def := o.strategies[key]
			if def == nil {
				continue
			}
			resultHas := keyIn(result, key)
			objHas := keyIn(om, key)
			if !resultHas && !objHas {
				continue
			}
			val, err := def.merge(result[key], om[key])
			if err != nil {
				return nil, &WrapperError{Key: key, Source: err}
			}
			if val != nil {
				result[key] = val
			}
		}
	}
	return result, nil
}

// Validate validates an object's keys per the schema. Mirrors JS
// ObjectSchema.validate(object).
func (o *ObjectSchema) Validate(object map[string]any) error {
	for _, key := range sortedKeys(object) {
		def := o.strategies[key]
		if def == nil {
			return &UnexpectedKeyError{Key: key}
		}
		if len(def.requires) > 0 {
			for _, other := range def.requires {
				if !keyIn(object, other) {
					return &MissingDependentKeysError{Key: key, Keys: def.requires}
				}
			}
		}
		if def.validate != nil {
			if err := def.validate(object[key]); err != nil {
				return &WrapperError{Key: key, Source: err}
			}
		}
	}
	for key := range o.requiredKeys {
		if !keyIn(object, key) {
			return &MissingKeyError{Key: key}
		}
	}
	return nil
}

func keyIn(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func resultKeys(result map[string]any, om map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	for k := range result {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range om {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

func sortedMapKeys(keys []string) []string {
	// dedupe + sort for deterministic iteration
	seen := map[string]bool{}
	var out []string
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isScalar reports whether a value cannot be an object schema (JS primitives).
func isScalar(v any) bool {
	switch v.(type) {
	case string, bool, int, int64, float64:
		return true
	}
	return false
}
