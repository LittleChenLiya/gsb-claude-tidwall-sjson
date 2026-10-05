package sjson

import (
	"fmt"
	"testing"
)

// expectCond runs a conditional set/delete and checks the exact output
// bytes. When the condition is not met the output must be byte-identical
// to the input.
func expectCond(t *testing.T, expect, got string, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if got != expect {
		t.Fatalf("expected '%v', got '%v'", expect, got)
	}
}

func TestCondMissing(t *testing.T) {
	// an existing null value rejects CondMissing
	got, err := SetIf(`{"a":null}`, "a", 1, CondMissing)
	expectCond(t, `{"a":null}`, got, err)
	// a missing path allows the update
	got, err = SetIf(`{"a":null}`, "b", 1, CondMissing)
	expectCond(t, `{"a":null,"b":1}`, got, err)
	// nested creation is allowed when the leaf is missing
	got, err = SetIf(`{"a":1}`, "b.c", 2, CondMissing)
	expectCond(t, `{"a":1,"b":{"c":2}}`, got, err)
	// an existing value of any type rejects CondMissing
	got, err = SetIf(`{"a":1}`, "a", 2, CondMissing)
	expectCond(t, `{"a":1}`, got, err)
}

func TestCondString(t *testing.T) {
	// CondString only allows replacing an original string
	got, err := SetIf(`{"a":"x","b":2}`, "a", "y", CondString)
	expectCond(t, `{"a":"y","b":2}`, got, err)
	// a number is not a string: unchanged
	got, err = SetIf(`{"a":"x","b":2}`, "b", "y", CondString)
	expectCond(t, `{"a":"x","b":2}`, got, err)
	// a missing path is not a string: no creation, unchanged
	got, err = SetIf(`{"a":1}`, "b.c", "x", CondString)
	expectCond(t, `{"a":1}`, got, err)
}

func TestCondNumericObjectKey(t *testing.T) {
	// object key "0" is a key, not an array index
	got, err := SetIf(`{"0":"x"}`, "0", "y", CondString)
	expectCond(t, `{"0":"y"}`, got, err)
	// the object holds a number at key "0": CondString rejects
	got, err = SetIf(`{"0":1}`, "0", "y", CondString)
	expectCond(t, `{"0":1}`, got, err)
	// array index 0 is an index
	got, err = SetIf(`["x",1]`, "0", "y", CondString)
	expectCond(t, `["y",1]`, got, err)
	// deleting through a parent object: "0" is a key of the object
	got, err = DeleteIf(`{"a":{"0":"x"}}`, "a.0", CondString)
	expectCond(t, `{"a":{}}`, got, err)
	// deleting through a parent array: 0 is an index of the array
	got, err = DeleteIf(`{"a":["x"]}`, "a.0", CondString)
	expectCond(t, `{"a":[]}`, got, err)
	// parent is an object but condition expects a number: unchanged
	got, err = DeleteIf(`{"a":{"0":"x"}}`, "a.0", CondNumber)
	expectCond(t, `{"a":{"0":"x"}}`, got, err)
}

func TestCondNull(t *testing.T) {
	got, err := SetIf(`{"a":null}`, "a", 5, CondNull)
	expectCond(t, `{"a":5}`, got, err)
	got, err = SetIf(`{"a":1}`, "a", 5, CondNull)
	expectCond(t, `{"a":1}`, got, err)
	// missing path is not null
	got, err = SetIf(`{"a":1}`, "b", 5, CondNull)
	expectCond(t, `{"a":1}`, got, err)
}

func TestCondTypes(t *testing.T) {
	const json = `{"b":true,"n":3.5,"s":"x","o":{"k":1},"r":[1],"j":{}}`
	got, err := SetIf(json, "b", false, CondBool)
	expectCond(t, `{"b":false,"n":3.5,"s":"x","o":{"k":1},"r":[1],"j":{}}`, got, err)
	got, err = SetIf(json, "n", 4, CondNumber)
	expectCond(t, `{"b":true,"n":4,"s":"x","o":{"k":1},"r":[1],"j":{}}`, got, err)
	got, err = SetRawIf(json, "o", `{"z":2}`, CondObject)
	expectCond(t, `{"b":true,"n":3.5,"s":"x","o":{"z":2},"r":[1],"j":{}}`, got, err)
	got, err = SetIf(json, "r", 9, CondArray)
	expectCond(t, `{"b":true,"n":3.5,"s":"x","o":{"k":1},"r":9,"j":{}}`, got, err)
	// CondJSON matches objects and arrays but not scalars
	got, err = SetIf(json, "j", 9, CondJSON)
	expectCond(t, `{"b":true,"n":3.5,"s":"x","o":{"k":1},"r":[1],"j":9}`, got, err)
	got, err = SetIf(json, "s", 9, CondJSON)
	expectCond(t, json, got, err)
	// CondExists matches any existing value
	got, err = SetIf(json, "s", 9, CondExists)
	expectCond(t, `{"b":true,"n":3.5,"s":9,"o":{"k":1},"r":[1],"j":{}}`, got, err)
	got, err = SetIf(json, "zzz", 9, CondExists)
	expectCond(t, json, got, err)
	// wrong types are rejected without changing bytes
	got, err = SetIf(json, "n", 4, CondBool)
	expectCond(t, json, got, err)
	got, err = SetIf(json, "o", 4, CondArray)
	expectCond(t, json, got, err)
}

func TestDeleteIf(t *testing.T) {
	got, err := DeleteIf(`{"a":{"b":1}}`, "a.b", CondNumber)
	expectCond(t, `{"a":{}}`, got, err)
	// delete a parent node: the parent type is checked at its location
	got, err = DeleteIf(`{"a":{"b":1}}`, "a", CondObject)
	expectCond(t, `{}`, got, err)
	got, err = DeleteIf(`{"a":{"b":1}}`, "a", CondString)
	expectCond(t, `{"a":{"b":1}}`, got, err)
	// deleting a missing path changes nothing
	got, err = DeleteIf(`{"a":1}`, "b", CondAny)
	expectCond(t, `{"a":1}`, got, err)
	// null can be deleted with CondNull, but not with CondMissing
	got, err = DeleteIf(`{"a":null,"b":1}`, "a", CondNull)
	expectCond(t, `{"b":1}`, got, err)
	got, err = DeleteIf(`{"a":null,"b":1}`, "a", CondMissing)
	expectCond(t, `{"a":null,"b":1}`, got, err)
}

func TestSetRawIf(t *testing.T) {
	got, err := SetRawIf(`{"a":1}`, "a", `{"x":2}`, CondNumber)
	expectCond(t, `{"a":{"x":2}}`, got, err)
	got, err = SetRawIf(`{"a":"s"}`, "a", `{"x":2}`, CondNumber)
	expectCond(t, `{"a":"s"}`, got, err)
	// invalid raw values are rejected first, before the condition
	got, err = SetRawIf(`{"a":1}`, "a", `{invalid`, CondAny)
	if err == nil {
		t.Fatal("expected error for invalid raw value")
	}
	if got != `{"a":1}` {
		t.Fatalf("expected input unchanged, got '%v'", got)
	}
	// invalid raw rejected even when the condition would fail anyway
	_, err = SetRawIf(`{"a":1}`, "a", `{invalid`, CondString)
	if err == nil {
		t.Fatal("expected error for invalid raw value")
	}
}

func TestCondBytesAndComplexPath(t *testing.T) {
	got, err := SetBytesIf([]byte(`{"a":"x"}`), "a", "y", CondString)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":"y"}` {
		t.Fatalf("got '%v'", got)
	}
	gotb, err := DeleteBytesIf([]byte(`{"a":null}`), "a", CondNull)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotb) != `{}` {
		t.Fatalf("got '%v'", gotb)
	}
	gotb, err = SetRawBytesIf([]byte(`{"a":1}`), "b", []byte(`[1,2]`),
		CondMissing)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotb) != `{"a":1,"b":[1,2]}` {
		t.Fatalf("got '%v'", gotb)
	}
	// conditions cannot be used with complex paths
	_, err = SetIf(`{"a":1}`, "a.#", 1, CondExists)
	if err == nil {
		t.Fatal("expected error for complex path with condition")
	}
}

// TestPlainSetUnchanged ensures the plain Set/Delete semantics are not
// affected by the conditional variants.
func TestPlainSetUnchanged(t *testing.T) {
	got, err := Set(`{"a":null}`, "a", 1)
	expectCond(t, `{"a":1}`, got, err)
	got, err = Set(`{"a":1}`, "b.c", 2)
	expectCond(t, `{"a":1,"b":{"c":2}}`, got, err)
	got, err = Delete(`{"a":{"0":"x"}}`, "a.0")
	expectCond(t, `{"a":{}}`, got, err)
}

// Example 1: set a value only when the path is missing.
func ExampleSetIf_missing() {
	got, _ := SetIf(`{"name":"Tom"}`, "age", 37, CondMissing)
	fmt.Println(got)
	got, _ = SetIf(`{"name":"Tom","age":null}`, "age", 37, CondMissing)
	fmt.Println(got)
	// Output:
	// {"name":"Tom","age":37}
	// {"name":"Tom","age":null}
}

// Example 2: replace a value only when the original is a string.
func ExampleSetIf_string() {
	got, _ := SetIf(`{"a":"x","b":2}`, "a", "y", CondString)
	fmt.Println(got)
	got, _ = SetIf(`{"a":"x","b":2}`, "b", "y", CondString)
	fmt.Println(got)
	// Output:
	// {"a":"y","b":2}
	// {"a":"x","b":2}
}

// Example 3: delete a value only when the original is null.
func ExampleDeleteIf() {
	got, _ := DeleteIf(`{"a":null,"b":1}`, "a", CondNull)
	fmt.Println(got)
	// Output: {"b":1}
}
