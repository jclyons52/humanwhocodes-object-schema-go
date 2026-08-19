# object-schema-go (humanwhocodes-object-schema)

Go port of [`@humanwhocodes/object-schema`](https://github.com/humanwhocodes/object-schema)
— an object definition/merging/validation schema, as used by ESLint's
flat-config / config-array machinery.

| | |
|---|---|
| Original size | ~460 LOC (src/) |
| Public API | `NewObjectSchema(definitions)`, `(*ObjectSchema).HasKey/Merge/Validate`, `MergeStrategies`, `ValidationStrategies`, plus the error types `UnexpectedKeyError`, `MissingKeyError`, `MissingDependentKeysError`, `WrapperError` |
| Module | `github.com/jclyons52/humanwhocodes-object-schema-go` |

## Files
- `object_schema.go` — the `ObjectSchema` type: constructor validates + normalizes
  definitions (string strategies, subschemas, `required` / `requires`), `HasKey`,
  `Merge(...objects)`, `Validate(object)`.
- `merge_strategy.go` — built-in merge strategies (`overwrite`, `replace`,
  `assign`) keyed by name, mirroring the JS `MergeStrategy` static class.
- `validation_strategy.go` — built-in validators (`array`, `boolean`, `number`,
  `object`, `object?`, `string`, `string!`).
- `errors.go` — the schema errors with JS-identical messages.
- `original/` — vendored original source (reference + parity oracle).
- `parity_test.go` — shells out to `node`, runs the real ObjectSchema over a
  26-case corpus (merge/validate across schemas, subschemas, required/requires,
  constructor errors), and requires identical results.

## Parity
```sh
go test ./...
# PARITY PASS: 26 cases, 0 mismatches (requires node)
```

Notes captured while porting:
- `Object.assign({}, v1, v2)` also indexes **strings/arrays** by position
  (`Object.assign({}, "a")` → `{"0":"a"}`), which a naive "copy maps only"
  port misses.
- A schema key with a `schema` subschema **overrides** its string
  `merge`/`validate` instead of merging the two (the JS constructor rewrites
  the key to subschema-driven functions).
