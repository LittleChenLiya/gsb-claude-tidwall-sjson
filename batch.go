package sjson

import (
	jsongo "encoding/json"
	"strconv"

	"github.com/tidwall/gjson"
)

// BatchOp describes a single set or delete operation that is executed as
// part of a batch. See SetBytesBatch for the batch semantics.
type BatchOp struct {
	// Path is a dot syntax path, such as "name.last" or "age".
	// An empty path targets the root document itself, which allows for
	// replacing the entire document. Deleting the root is not allowed.
	Path string
	// Value is the value to set. It is ignored when Delete is true.
	// Any value accepted by SetBytes may be used. When Raw is true the
	// value must be a string or []byte containing a valid JSON block.
	Value interface{}
	// Raw indicates that Value is a raw, premarshalled JSON block
	// (string or []byte) instead of a Go value. The block is validated
	// and the batch fails when it is not valid JSON.
	Raw bool
	// Delete indicates that the path should be deleted instead of set.
	Delete bool
}

// BatchSet returns a BatchOp that sets path to value.
func BatchSet(path string, value interface{}) BatchOp {
	return BatchOp{Path: path, Value: value}
}

// BatchSetRaw returns a BatchOp that sets path to a raw JSON block.
// The block must be valid JSON or the whole batch fails.
func BatchSetRaw(path, raw string) BatchOp {
	return BatchOp{Path: path, Value: raw, Raw: true}
}

// BatchDelete returns a BatchOp that deletes the value at path.
func BatchDelete(path string) BatchOp {
	return BatchOp{Path: path, Delete: true}
}

// BatchInterval is a half-open byte interval [Start, End) into a JSON
// document. A zero-width interval (Start == End) marks an insertion or
// collapse point.
type BatchInterval struct {
	Start int
	End   int
}

// BatchOpResult describes the effect of one executed batch operation.
//
// Before and After are the minimal changed regions of the document as it
// was immediately before and immediately after the operation:
//
//   - For a set of an existing value, Before covers the old value bytes
//     and After covers the new value bytes.
//   - For a set that creates a new member, Before is a zero-width
//     interval at the insertion point and After covers the inserted
//     key/value bytes.
//   - For a delete, Before covers the removed bytes (including the
//     separating comma or key) and After is a zero-width interval at the
//     collapse point.
//   - For a root replacement, Before and After span the whole document
//     before and after the operation.
//
// Because every operation is applied to the document produced by the
// previous operation, all intervals are always relative to the document
// version they refer to; earlier length changes never leak stale offsets
// into later operations.
type BatchOpResult struct {
	Op     BatchOp       // the operation that was executed
	Before BatchInterval // changed region in the pre-operation document
	After  BatchInterval // changed region in the post-operation document
}

// BatchResult is the outcome of SetBytesBatch or SetBatch.
type BatchResult struct {
	// JSON is the final document after all operations. When the batch
	// fails, JSON is the complete original input, unchanged.
	JSON []byte
	// Ops holds one entry per executed operation, in execution order.
	// When the batch fails, Ops contains only the operations that were
	// executed before the failing one.
	Ops []BatchOpResult
}

// BatchError is returned when one operation of a batch fails. The batch
// is atomic: no partial result is published and BatchResult.JSON holds
// the complete original input.
type BatchError struct {
	Index int    // index of the failed operation within the batch
	Path  string // path of the failed operation
	Err   error  // underlying error
}

func (e *BatchError) Error() string {
	return "sjson: batch operation " + strconv.Itoa(e.Index) +
		" failed at path " + strconv.Quote(e.Path) + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *BatchError) Unwrap() error { return e.Err }

// SetBatch executes a batch of set and delete operations against json.
// It behaves like SetBytesBatch but works with strings.
func SetBatch(json string, ops []BatchOp) (BatchResult, error) {
	return SetBytesBatch([]byte(json), ops)
}

// SetBytesBatch executes a batch of set and delete operations, in order,
// against json.
//
// Each operation is applied to the document produced by the previous
// operation, reusing the same raw byte splicing as SetBytes and
// DeleteBytes. A later operation therefore always resolves its path
// against the current structure: if an earlier operation changed the
// document length or shifted array indices, the next operation is
// relocated against the new document and never reuses stale offsets.
//
// The batch is atomic. If any operation has an invalid path or a value
// that is not valid JSON, a *BatchError is returned and BatchResult.JSON
// holds the complete original input; no partial result is published.
//
// The input json bytes are never modified.
func SetBytesBatch(json []byte, ops []BatchOp) (BatchResult, error) {
	res := BatchResult{JSON: json}
	cur := json
	for i := range ops {
		next, err := applyBatchOp(cur, &ops[i])
		if err != nil {
			res.JSON = json
			return res, &BatchError{Index: i, Path: ops[i].Path, Err: err}
		}
		before, after := changedRegion(cur, next)
		res.Ops = append(res.Ops, BatchOpResult{
			Op:     ops[i],
			Before: before,
			After:  after,
		})
		cur = next
	}
	res.JSON = cur
	return res, nil
}

// applyBatchOp applies a single batch operation to json using the
// existing raw byte modification functions.
func applyBatchOp(json []byte, op *BatchOp) ([]byte, error) {
	if op.Delete {
		if op.Path == "" {
			return nil, &errorType{"cannot delete the root document"}
		}
		return DeleteBytes(json, op.Path)
	}
	if op.Path == "" {
		// Root replacement: the whole document is swapped for the new
		// value, which must be valid JSON.
		raw, err := batchRawValue(op)
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	if op.Raw {
		raw, err := batchRawValue(op)
		if err != nil {
			return nil, err
		}
		return SetRawBytes(json, op.Path, raw)
	}
	return SetBytes(json, op.Path, op.Value)
}

// batchRawValue returns the raw JSON representation of the operation
// value and validates that it is well-formed JSON.
func batchRawValue(op *BatchOp) ([]byte, error) {
	var raw []byte
	if op.Raw {
		switch v := op.Value.(type) {
		case string:
			raw = []byte(v)
		case []byte:
			raw = v
		default:
			return nil, &errorType{
				"raw batch value must be a string or []byte"}
		}
	} else {
		b, err := jsongo.Marshal(op.Value)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	if !gjson.ValidBytes(raw) {
		return nil, &errorType{"batch value is not valid JSON"}
	}
	return raw, nil
}

// changedRegion returns the minimal changed regions of two document
// versions as half-open byte intervals, computed from the longest common
// prefix and suffix.
func changedRegion(before, after []byte) (BatchInterval, BatchInterval) {
	n, m := len(before), len(after)
	p := 0
	for p < n && p < m && before[p] == after[p] {
		p++
	}
	q := 0
	for q < n-p && q < m-p && before[n-1-q] == after[m-1-q] {
		q++
	}
	return BatchInterval{Start: p, End: n - q},
		BatchInterval{Start: p, End: m - q}
}
