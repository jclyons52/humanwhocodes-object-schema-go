package humanwhocodesobjectschema

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Entry is one parity case: an op plus its payload. Both this test and the
// JS driver (node + the real ObjectSchema) compute a result per Entry so the
// two compare 1:1. Results are canonical-JSON strings (or "ERR:<message>").
type Entry struct {
	Op      string         `json:"op"` // "merge" | "validate"
	Def     map[string]any `json:"def,omitempty"`
	Objects []any          `json:"objects,omitempty"`
	Object  map[string]any `json:"object"`
}

func buildCorpus() []Entry {
	var es []Entry
	// A few schemas mixing string strategies, subschemas, required/requires.
	type sch struct {
		name   string
		def    map[string]any
		objs   [][]any          // for merge
		inputs []map[string]any // for validate
	}
	schemas := []sch{
		{
			name: "basic",
			def: map[string]any{
				"name":  map[string]any{"merge": "assign", "validate": "string!"},
				"count": map[string]any{"merge": "overwrite", "validate": "number"},
				"tags":  map[string]any{"merge": "replace", "validate": "array"},
				"flag":  map[string]any{"merge": "overwrite", "validate": "boolean"},
			},
			objs: [][]any{
				{map[string]any{"name": "a", "count": 1}, map[string]any{"count": 2, "tags": []any{"x"}}},
				{map[string]any{"name": "a"}, map[string]any{"name": "b", "count": 5}},
				{map[string]any{"count": 1}, map[string]any{"flag": true, "name": "z"}},
			},
			inputs: []map[string]any{
				{"name": "ok", "count": 3},
				{"count": "bad-type"},
				{"name": ""},
				{"unknown": 1},
			},
		},
		{
			name: "required",
			def: map[string]any{
				"req": map[string]any{"required": true, "validate": "string", "merge": "replace"},
				"opt": map[string]any{"required": false, "merge": "assign", "validate": "object?"},
			},
			objs: [][]any{
				{map[string]any{"req": "x", "opt": map[string]any{"k": 1}}, map[string]any{"opt": map[string]any{"k2": 2}}},
			},
			inputs: []map[string]any{
				{"req": "x"},
				{"opt": nil},
				{},
			},
		},
		{
			name: "requires-dep",
			def: map[string]any{
				"a": map[string]any{"required": true, "validate": "number", "merge": "overwrite"},
				"b": map[string]any{"validate": "string", "requires": []any{"a"}, "merge": "replace"},
			},
			objs: [][]any{
				{map[string]any{"a": 1}, map[string]any{"b": "s"}},
			},
			inputs: []map[string]any{
				{"a": 1, "b": "s"},
				{"b": "s"}, // missing dependency
			},
		},
		{
			name: "subschema",
			def: map[string]any{
				"nested": map[string]any{
					"merge":    "assign",
					"validate": "object",
					"schema": map[string]any{
						"x": map[string]any{"merge": "assign", "validate": "number"},
						"y": map[string]any{"merge": "replace", "validate": "object?"},
					},
				},
				"strategy-bad": map[string]any{"merge": "nope", "validate": "string"}, // constructor error
			},
			objs: [][]any{
				{map[string]any{"nested": map[string]any{"x": 1}}, map[string]any{"nested": map[string]any{"x": 2, "y": nil}}},
			},
			inputs: []map[string]any{
				{"nested": map[string]any{"x": 1}},
				{"nested": map[string]any{"x": "not-anumber"}},
				{"nested": 5},
			},
		},
		{
			name: "three-way",
			def: map[string]any{
				"mode": map[string]any{"merge": "overwrite", "validate": "string"},
				"opts": map[string]any{"merge": "assign", "validate": "object?"},
			},
			objs: [][]any{
				{
					map[string]any{"mode": "a", "opts": map[string]any{"x": 1}},
					map[string]any{"opts": map[string]any{"y": 2}},
					map[string]any{"mode": "c", "opts": map[string]any{"z": 3}},
				},
			},
			inputs: []map[string]any{
				{"mode": "x", "opts": nil},
				{"opts": []any{1, 2}}, // array passes typeof object + object?
			},
		},
		{
			name: "constructor-errors",
			def: map[string]any{
				"bad-schema": map[string]any{"schema": "notanobject"},
			},
		},
		{
			name: "array-merge",
			def: map[string]any{
				"list": map[string]any{"merge": "replace", "validate": "array"},
			},
			objs: [][]any{
				{map[string]any{"list": []any{1, 2}}, map[string]any{"list": []any{3}}},
			},
			inputs: []map[string]any{
				{"list": []any{1, 2, 3}},
				{"list": "notarray"},
			},
		},
	}
	for _, s := range schemas {
		for _, objs := range s.objs {
			es = append(es, Entry{Op: "merge", Def: s.def, Objects: objs})
		}
		for _, in := range s.inputs {
			es = append(es, Entry{Op: "validate", Def: s.def, Object: in})
		}
		if len(s.objs) == 0 && len(s.inputs) == 0 {
			// constructor-only schema: exercise it via a trivial merge/validate
			es = append(es, Entry{Op: "merge", Def: s.def, Objects: []any{map[string]any{}, map[string]any{}}})
			es = append(es, Entry{Op: "validate", Def: s.def, Object: map[string]any{}})
		}
	}
	return es
}

// goResult computes the Go port's answer for an entry: canonical JSON or
// "ERR:<message>".
func goResult(e Entry) any {
	os, err := NewObjectSchema(e.Def)
	if err != nil {
		return "ERR:" + err.Error()
	}
	switch e.Op {
	case "merge":
		objs := make([]any, 0, len(e.Objects))
		objs = append(objs, e.Objects...)
		res, err := os.Merge(objs...)
		if err != nil {
			return "ERR:" + err.Error()
		}
		return canon(res)
	case "validate":
		if err := os.Validate(e.Object); err != nil {
			return "ERR:" + err.Error()
		}
		return ""
	}
	return "UNKNOWN_OP:" + e.Op
}

// canon marshals a value as JSON with recursively sorted map keys.
func canon(v any) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			b.Write(kb)
			b.WriteByte(':')
			b.WriteString(canon(t[k]))
		}
		b.WriteByte('}')
		return b.String()
	case []any:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = canon(x)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

const jsDriver = `'use strict';
const fs = require('fs');
const corpus = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const orig = require(process.env.PORT_ORIG);
function stable(obj) {
  if (obj === null || typeof obj !== 'object') return JSON.stringify(obj);
  if (Array.isArray(obj)) return '[' + obj.map(stable).join(',') + ']';
  return '{' + Object.keys(obj).sort().map(k => JSON.stringify(k) + ':' + stable(obj[k])).join(',') + '}';
}
function r(e) {
  try {
    const os = new orig.ObjectSchema(e.def);
    if (e.op === 'merge') {
      const merged = os.merge(...e.objects);
      return stable(merged);
    }
    os.validate(e.object);
    return '';
  } catch (err) {
    return 'ERR:' + (err && err.message ? err.message : String(err));
  }
}
const out = [];
for (const e of corpus) out.push(r(e));
fs.writeFileSync(process.argv[3], JSON.stringify(out));
`

func TestParity(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping JS parity")
	}
	orig, err := filepath.Abs("original")
	if err != nil {
		t.Fatal(err)
	}
	corpus := buildCorpus()

	dir := t.TempDir()
	corpusJSON, _ := json.Marshal(corpus)
	if err := os.WriteFile(filepath.Join(dir, "corpus.json"), corpusJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "driver.js"), []byte(jsDriver), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(nodeBin, "driver.js", "corpus.json", "result.json")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PORT_ORIG="+orig)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node driver failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var jsResults []any
	if err := json.Unmarshal(raw, &jsResults); err != nil {
		t.Fatalf("parse node result: %v", err)
	}
	if len(jsResults) != len(corpus) {
		t.Fatalf("result length %d != corpus %d", len(jsResults), len(corpus))
	}
	mismatch := 0
	limit := 40
	for i, e := range corpus {
		got := stringify(goResult(e))
		want := stringify(jsResults[i])
		if got != want {
			mismatch++
			if mismatch <= limit {
				t.Errorf("op=%s def=%s payload=%v\ngo =%v\njs =%v", e.Op, canon(e.Def), key(e), got, jsResults[i])
			}
		}
	}
	t.Logf("parity: %d cases, %d mismatches", len(corpus), mismatch)
	if mismatch > 0 {
		t.Fatalf("parity mismatch (%d/%d) vs real ObjectSchema", mismatch, len(corpus))
	}
}

func stringify(v any) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	switch t := v.(type) {
	case bool:
		if t {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return canon(v)
	}
}

func key(e Entry) string { return e.Op + ":" + canon(e.Def) }
