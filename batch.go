package sjson

import (
	jsongo "encoding/json"
	"unsafe"

	"github.com/tidwall/gjson"
)

// OpKind identifies the type of a batch operation.
type OpKind int

const (
	// OpSet sets a value at a path.
	OpSet OpKind = iota
	// OpDelete deletes the value at a path.
	OpDelete
)

// BatchOp is a single path set/delete operation performed in order.
//
// A set operation replaces the value at Path with Value. A delete operation
// removes the value at Path. Operations are applied one after another; each
// operation re-locates its target in the document produced by the previous
// operation, so array indices and offsets never carry over.
//
// Path is a dot-notation path such as "name.last" or "children.1". An empty
// path means the root document: a root set replaces the entire document.
//
// For set operations, string and []byte values are treated as raw JSON text
// (validated with gjson.Valid); other types are marshaled with
// encoding/json. A string value must therefore include its quotes, e.g.
// SetOp("name", `"Tom"`).
type BatchOp struct {
	// Kind is the operation type.
	Kind OpKind
	// Path is the dot-notation path. An empty path means the root document.
	Path string
	// Value is the set value. It is ignored for OpDelete.
	Value interface{}
}

// SetOp creates a set operation that replaces the value at path with value.
// string and []byte values are treated as raw JSON text; other types are
// marshaled with encoding/json.
func SetOp(path string, value interface{}) BatchOp {
	return BatchOp{Kind: OpSet, Path: path, Value: value}
}

// DeleteOp creates a delete operation that removes the value at path.
func DeleteOp(path string) BatchOp {
	return BatchOp{Kind: OpDelete, Path: path}
}

// Span is a half-open byte range [Start, End) in a JSON document.
type Span struct {
	Start int
	End   int
}

// Len returns the number of bytes in the span.
func (s Span) Len() int { return s.End - s.Start }

// OpResult describes one executed operation's effect on the document.
//
// Before is the byte range replaced in the input document of the operation;
// After is the byte range occupied by the replacement text in the output
// document. A zero-length Before means a newly inserted value; a zero-length
// After means a deletion.
type OpResult struct {
	Kind   OpKind
	Path   string
	Before Span
	After  Span
}

// SetBatch applies a sequence of path set/delete operations to json in order.
//
// Each operation is re-located in the document produced by the previous one,
// so changing the length of a value or deleting an array element does not
// invalidate later paths. If any operation has an invalid path or a value
// that is not valid JSON, the whole batch is aborted: the returned bytes are
// the original input and no result is published.
//
// The input bytes are never modified. The returned results describe the
// before/after byte ranges of each executed operation.
func SetBatch(json []byte, ops ...BatchOp) ([]byte, []OpResult, error) {
	for i := range ops {
		if err := validateOp(ops[i]); err != nil {
			return json, nil, err
		}
	}
	cur := string(json)
	results := make([]OpResult, 0, len(ops))
	for _, op := range ops {
		var out []byte
		var before, after Span
		var err error
		if op.Path == "" {
			out, before, after, err = applyRoot(cur, op)
		} else {
			out, before, after, err = applyPath(cur, op)
		}
		if err != nil {
			return json, nil, err
		}
		cur = string(out)
		results = append(results, OpResult{
			Kind:   op.Kind,
			Path:   op.Path,
			Before: before,
			After:  after,
		})
	}
	return []byte(cur), results, nil
}

// SetBatchString is like SetBatch but accepts and returns a string.
func SetBatchString(json string, ops ...BatchOp) (string, []OpResult, error) {
	res, results, err := SetBatch([]byte(json), ops...)
	return string(res), results, err
}

func validateOp(op BatchOp) error {
	if op.Path == "" {
		if op.Kind == OpDelete {
			return &errorType{"cannot delete root"}
		}
		return validateValue(op.Value)
	}
	r, simple := parsePath(op.Path)
	if !simple {
		if op.Kind == OpDelete {
			return &errorType{"cannot delete value from a complex path"}
		}
		return &errorType{"complex paths are not supported in batch"}
	}
	_ = r
	if op.Kind == OpSet {
		return validateValue(op.Value)
	}
	return nil
}

func validateValue(v interface{}) error {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if !gjson.Valid(t) {
			return &errorType{"value is not valid JSON"}
		}
		return nil
	case []byte:
		s := *(*string)(unsafe.Pointer(&t))
		if !gjson.Valid(s) {
			return &errorType{"value is not valid JSON"}
		}
		return nil
	default:
		b, err := jsongo.Marshal(v)
		if err != nil {
			return err
		}
		if !gjson.ValidBytes(b) {
			return &errorType{"value is not valid JSON"}
		}
		return nil
	}
}

func valueToRaw(v interface{}) (raw string, err error) {
	switch t := v.(type) {
	case nil:
		return "null", nil
	case string:
		if !gjson.Valid(t) {
			return "", &errorType{"value is not valid JSON"}
		}
		return t, nil
	case []byte:
		s := *(*string)(unsafe.Pointer(&t))
		if !gjson.Valid(s) {
			return "", &errorType{"value is not valid JSON"}
		}
		return s, nil
	default:
		b, err := jsongo.Marshal(v)
		if err != nil {
			return "", err
		}
		return *(*string)(unsafe.Pointer(&b)), nil
	}
}

func applyPath(jstr string, op BatchOp) (out []byte, before, after Span, err error) {
	var raw string
	if op.Kind == OpSet {
		raw, err = valueToRaw(op.Value)
		if err != nil {
			return nil, before, after, err
		}
	}
	r, simple := parsePath(op.Path)
	if !simple {
		return nil, before, after, &errorType{"complex paths are not supported in batch"}
	}
	paths := []pathResult{r}
	for r.more {
		r, simple = parsePath(r.path)
		if !simple {
			return nil, before, after, &errorType{"complex paths are not supported in batch"}
		}
		paths = append(paths, r)
	}
	out, sp, err := appendRawPaths(nil, jstr, paths, raw, false, op.Kind == OpDelete)
	if err != nil {
		return nil, before, after, err
	}
	before = Span{Start: sp.index, End: sp.index + sp.length}
	after = Span{Start: sp.postIndex, End: sp.postIndex + sp.postLen}
	return out, before, after, nil
}

func applyRoot(jstr string, op BatchOp) (out []byte, before, after Span, err error) {
	if op.Kind == OpDelete {
		return nil, before, after, &errorType{"cannot delete root"}
	}
	raw, err := valueToRaw(op.Value)
	if err != nil {
		return nil, before, after, err
	}
	out = []byte(raw)
	before = Span{Start: 0, End: len(jstr)}
	after = Span{Start: 0, End: len(raw)}
	return out, before, after, nil
}
