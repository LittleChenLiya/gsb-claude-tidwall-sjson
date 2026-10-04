package sjson

import (
	"strings"
	"testing"
)

// Deleting array element 0 shifts the remaining elements down, so a
// following set of the new index 1 must modify the original element 2.
func TestBatchDeleteThenSetRelocates(t *testing.T) {
	in := []byte(`{"arr":["a","b","c"]}`)
	res, err := SetBytesBatch(in, []BatchOp{
		BatchDelete("arr.0"),
		BatchSet("arr.1", "z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(res.JSON), `{"arr":["b","z"]}`; got != want {
		t.Fatalf("expected '%v', got '%v'", want, got)
	}
	if len(res.Ops) != 2 {
		t.Fatalf("expected 2 op results, got %d", len(res.Ops))
	}
	if string(in) != `{"arr":["a","b","c"]}` {
		t.Fatal("input bytes were modified")
	}
}

// After the first operation replaces a short string with a much longer
// one, the offsets reported for the second operation must still be
// accurate against the relocated document.
func TestBatchOffsetsAccurateAfterGrowth(t *testing.T) {
	in := []byte(`{"a":"x","b":"y"}`)
	long := "a-much-longer-string-than-before"
	res, err := SetBytesBatch(in, []BatchOp{
		BatchSet("a", long),
		BatchSet("b", "zz"),
	})
	if err != nil {
		t.Fatal(err)
	}
	mid := `{"a":"` + long + `","b":"y"}`
	want := `{"a":"` + long + `","b":"zz"}`
	if string(res.JSON) != want {
		t.Fatalf("expected '%v', got '%v'", want, string(res.JSON))
	}
	if len(res.Ops) != 2 {
		t.Fatalf("expected 2 op results, got %d", len(res.Ops))
	}
	// op 1: "x" -> long string
	op1 := res.Ops[0]
	if got := string(in[op1.Before.Start:op1.Before.End]); got != "x" {
		t.Fatalf("op1 before region = %q, want %q", got, "x")
	}
	if got := mid[op1.After.Start:op1.After.End]; got != long {
		t.Fatalf("op1 after region = %q, want %q", got, long)
	}
	// op 2: its intervals must be valid in the relocated documents,
	// not in the original input.
	op2 := res.Ops[1]
	if got := mid[op2.Before.Start:op2.Before.End]; got != "y" {
		t.Fatalf("op2 before region = %q, want %q", got, "y")
	}
	if got := want[op2.After.Start:op2.After.End]; got != "zz" {
		t.Fatalf("op2 after region = %q, want %q", got, "zz")
	}
	if op2.Before.Start != strings.Index(mid, `"y"`)+1 {
		t.Fatalf("op2 before start = %d, want %d",
			op2.Before.Start, strings.Index(mid, `"y"`)+1)
	}
}

// A failing third operation (value is not valid JSON) must roll the
// whole batch back to the complete initial input.
func TestBatchFailureRestoresInitialInput(t *testing.T) {
	in := []byte(`{"a":1,"b":[1,2,3]}`)
	res, err := SetBytesBatch(in, []BatchOp{
		BatchSet("a", 2),
		BatchSetRaw("b.0", `{"ok":true}`),
		BatchSetRaw("b.1", `{invalid`),
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	berr, ok := err.(*BatchError)
	if !ok {
		t.Fatalf("expected *BatchError, got %T", err)
	}
	if berr.Index != 2 {
		t.Fatalf("expected failing op index 2, got %d", berr.Index)
	}
	if string(res.JSON) != string(in) {
		t.Fatalf("expected original input %q, got %q", string(in),
			string(res.JSON))
	}
	if len(res.Ops) != 2 {
		t.Fatalf("expected 2 executed ops, got %d", len(res.Ops))
	}
	if string(in) != `{"a":1,"b":[1,2,3]}` {
		t.Fatal("input bytes were modified")
	}
}

// An invalid path must also abort the whole batch.
func TestBatchInvalidPathFails(t *testing.T) {
	in := []byte(`{"a":1}`)
	res, err := SetBytesBatch(in, []BatchOp{
		BatchSet("a", 2),
		BatchDelete("a.#"), // complex paths cannot be deleted
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, ok := err.(*BatchError); !ok {
		t.Fatalf("expected *BatchError, got %T", err)
	}
	if string(res.JSON) != string(in) {
		t.Fatalf("expected original input, got %q", string(res.JSON))
	}
}

// Escaped keys containing dots must resolve to the literal key.
func TestBatchEscapedKeys(t *testing.T) {
	in := []byte(`{"a.b":{"c.d":1}}`)
	res, err := SetBytesBatch(in, []BatchOp{
		BatchSet(`a\.b.c\.d`, 2),
		BatchSet(`a\.b.e`, "new"),
		BatchDelete(`a\.b.c\.d`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(res.JSON), `{"a.b":{"e":"new"}}`; got != want {
		t.Fatalf("expected '%v', got '%v'", want, got)
	}
}

// An empty path targets the root document and replaces it wholesale.
func TestBatchRootReplacement(t *testing.T) {
	in := []byte(`{"old":true}`)
	res, err := SetBytesBatch(in, []BatchOp{
		BatchSetRaw("", `[1,2,3]`),
		BatchSet("1", 9),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(res.JSON), `[1,9,3]`; got != want {
		t.Fatalf("expected '%v', got '%v'", want, got)
	}
	// the root replacement spans the whole document on both sides
	op0 := res.Ops[0]
	if op0.Before.Start != 0 || op0.Before.End != len(in) {
		t.Fatalf("op0 before = %+v, want [0,%d)", op0.Before, len(in))
	}
	if op0.After.Start != 0 || op0.After.End != len(`[1,2,3]`) {
		t.Fatalf("op0 after = %+v, want [0,7)", op0.After)
	}
	// a non-raw root value is marshalled to JSON
	res, err = SetBytesBatch(in, []BatchOp{BatchSet("", 42)})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(res.JSON); got != `42` {
		t.Fatalf("expected '42', got '%v'", got)
	}
	// an invalid raw root value fails the batch
	res, err = SetBytesBatch(in, []BatchOp{BatchSetRaw("", `nope`)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if string(res.JSON) != string(in) {
		t.Fatal("batch did not restore the original input")
	}
	// the root cannot be deleted
	if _, err = SetBytesBatch(in, []BatchOp{BatchDelete("")}); err == nil {
		t.Fatal("expected an error")
	}
}

// The string variant behaves like the bytes variant.
func TestSetBatchString(t *testing.T) {
	res, err := SetBatch(`{"a":1}`, []BatchOp{
		BatchSet("a", 2),
		BatchSet("b.c", "hi"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(res.JSON), `{"a":2,"b":{"c":"hi"}}`; got != want {
		t.Fatalf("expected '%v', got '%v'", want, got)
	}
}

// The default SetBytes behavior is unchanged by the batch API.
func TestBatchSetBytesCompat(t *testing.T) {
	out, err := SetBytes([]byte(`{"a":1}`), "a", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != `{"a":2}` {
		t.Fatalf("expected '{\"a\":2}', got '%v'", got)
	}
}
