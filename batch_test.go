package sjson

import (
	"testing"

	"github.com/tidwall/gjson"
)

// TestBatchDeleteThenReindex verifies that after deleting array item 0,
// the next set on "1" modifies the original item 2 (the new item 1),
// not the original item 1. Offsets must be recomputed, not reused.
func TestBatchDeleteThenReindex(t *testing.T) {
	json := []byte(`["a","b","c"]`)
	orig := append([]byte(nil), json...)
	out, results, err := SetBatch(json,
		DeleteOp("0"),
		SetOp("1", `"B"`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `["b","B"]` {
		t.Fatalf("expected [\"b\",\"B\"], got %s", out)
	}
	if string(json) != string(orig) {
		t.Fatal("original input was modified")
	}
	if gjson.Get(string(out), "0").String() != "b" {
		t.Fatal("orig item 1 should be unchanged")
	}
	if gjson.Get(string(out), "1").String() != "B" {
		t.Fatal("orig item 2 should be replaced")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if got := string(orig[results[0].Before.Start:results[0].Before.End]); got != `"a",` {
		t.Fatalf("op1 before = %q, want `\"a\",`", got)
	}
	if results[0].After.Start != results[0].After.End {
		t.Fatalf("op1 after should be zero-length, got %+v", results[0].After)
	}
	intermediate := []byte(`["b","c"]`)
	if got := string(intermediate[results[1].Before.Start:results[1].Before.End]); got != `"c"` {
		t.Fatalf("op2 before = %q, want `\"c\"`", got)
	}
	if got := string(out[results[1].After.Start:results[1].After.End]); got != `"B"` {
		t.Fatalf("op2 after = %q, want `\"B\"`", got)
	}
}

// TestBatchLengthChange verifies that after changing a short string to a
// long string, the second set's offset is still accurate.
func TestBatchLengthChange(t *testing.T) {
	json := []byte(`{"a":"short","b":"end"}`)
	orig := append([]byte(nil), json...)
	out, results, err := SetBatch(json,
		SetOp("a", `"a much longer string"`),
		SetOp("b", `"done"`),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"a much longer string","b":"done"}`
	if string(out) != want {
		t.Fatalf("expected %s, got %s", want, out)
	}
	if string(json) != string(orig) {
		t.Fatal("original input was modified")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if got := string(orig[results[0].Before.Start:results[0].Before.End]); got != `"short"` {
		t.Fatalf("op1 before = %q", got)
	}
	if got := string(out[results[0].After.Start:results[0].After.End]); got != `"a much longer string"` {
		t.Fatalf("op1 after = %q", got)
	}
	intermediate := []byte(`{"a":"a much longer string","b":"end"}`)
	if got := string(intermediate[results[1].Before.Start:results[1].Before.End]); got != `"end"` {
		t.Fatalf("op2 before = %q, want `\"end\"`", got)
	}
	if got := string(out[results[1].After.Start:results[1].After.End]); got != `"done"` {
		t.Fatalf("op2 after = %q", got)
	}
}

// TestBatchFailureRollback verifies that when the third step has an invalid
// value, the whole batch is aborted and the original input is returned.
func TestBatchFailureRollback(t *testing.T) {
	json := []byte(`{"a":1,"b":2}`)
	orig := append([]byte(nil), json...)
	out, results, err := SetBatch(json,
		SetOp("a", `10`),
		SetOp("b", `20`),
		SetOp("c", `{bad`),
	)
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
	if string(out) != string(orig) {
		t.Fatalf("expected rollback to %q, got %q", orig, out)
	}
	if results != nil {
		t.Fatal("expected no results on failure")
	}
	if string(json) != string(orig) {
		t.Fatal("original input was modified")
	}
}

// TestBatchFailureInvalidPath verifies that an invalid path aborts the batch.
func TestBatchFailureInvalidPath(t *testing.T) {
	json := []byte(`{"a":1}`)
	orig := append([]byte(nil), json...)
	out, _, err := SetBatch(json,
		SetOp("a", `2`),
		SetOp("b", `3`),
		DeleteOp("a.#(x>1)"),
	)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
	if string(out) != string(orig) {
		t.Fatalf("expected rollback, got %q", out)
	}
}

// TestBatchEscapedKey verifies that escaped keys work in batch operations.
func TestBatchEscapedKey(t *testing.T) {
	json := []byte(`{"app.token":"abc"}`)
	out, _, err := SetBatch(json,
		SetOp(`app\.token`, `"xyz"`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"app.token":"xyz"}` {
		t.Fatalf("unexpected: %s", out)
	}
	if gjson.Get(string(out), `app\.token`).String() != "xyz" {
		t.Fatal("escaped key not set")
	}
}

// TestBatchRootReplacement verifies that an empty path replaces the root.
func TestBatchRootReplacement(t *testing.T) {
	json := []byte(`{"a":1}`)
	orig := append([]byte(nil), json...)
	out, results, err := SetBatch(json,
		SetOp("", `"newroot"`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"newroot"` {
		t.Fatalf("unexpected: %s", out)
	}
	if string(json) != string(orig) {
		t.Fatal("original input was modified")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Before.Start != 0 || results[0].Before.End != len(json) {
		t.Fatalf("bad before range: %+v", results[0].Before)
	}
	if results[0].After.Start != 0 || results[0].After.End != len(`"newroot"`) {
		t.Fatalf("bad after range: %+v", results[0].After)
	}
}

// TestBatchEmpty verifies that an empty batch returns the original unchanged.
func TestBatchEmpty(t *testing.T) {
	json := []byte(`{"a":1}`)
	out, results, err := SetBatch(json)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(json) {
		t.Fatal("empty batch should return original")
	}
	if len(results) != 0 {
		t.Fatal("expected no results")
	}
}

// TestBatchInsertRanges verifies the before/after ranges for a new key.
func TestBatchInsertRanges(t *testing.T) {
	json := []byte(`{"a":1}`)
	out, results, err := SetBatch(json,
		SetOp("b", `2`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a":1,"b":2}` {
		t.Fatalf("unexpected: %s", out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Before.Start != results[0].Before.End {
		t.Fatalf("expected zero-length before, got %+v", results[0].Before)
	}
	if results[0].Before.Start != 6 {
		t.Fatalf("expected insertion at 6, got %d", results[0].Before.Start)
	}
	if got := string(out[results[0].After.Start:results[0].After.End]); got != `,"b":2` {
		t.Fatalf("after = %q, want `,\"b\":2`", got)
	}
}

// TestBatchDeleteRanges verifies the before/after ranges for a delete.
func TestBatchDeleteRanges(t *testing.T) {
	json := []byte(`{"a":1,"b":2}`)
	out, results, err := SetBatch(json,
		DeleteOp("a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"b":2}` {
		t.Fatalf("unexpected: %s", out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if got := string(json[results[0].Before.Start:results[0].Before.End]); got != `"a":1,` {
		t.Fatalf("before = %q, want `\"a\":1,`", got)
	}
	if results[0].After.Start != results[0].After.End {
		t.Fatalf("expected zero-length after, got %+v", results[0].After)
	}
}

// TestBatchString verifies the string convenience function.
func TestBatchString(t *testing.T) {
	json := `{"a":1}`
	out, results, err := SetBatchString(json,
		SetOp("a", `2`),
		SetOp("b", `3`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"a":2,"b":3}` {
		t.Fatalf("unexpected: %s", out)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestBatchTypedValues verifies that non-string values are marshaled.
func TestBatchTypedValues(t *testing.T) {
	json := []byte(`{"a":1}`)
	out, _, err := SetBatch(json,
		SetOp("a", 42),
		SetOp("b", true),
		SetOp("c", nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a":42,"b":true,"c":null}` {
		t.Fatalf("unexpected: %s", out)
	}
}

// TestBatchDeleteMissing verifies that deleting a non-existent path errors.
func TestBatchDeleteMissing(t *testing.T) {
	json := []byte(`{"a":1}`)
	orig := append([]byte(nil), json...)
	out, _, err := SetBatch(json,
		DeleteOp("b"),
	)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if string(out) != string(orig) {
		t.Fatalf("expected rollback, got %q", out)
	}
}
