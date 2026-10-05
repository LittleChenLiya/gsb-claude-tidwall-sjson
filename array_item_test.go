package sjson

import (
	"strings"
	"testing"
)

// Run the targeted tests for the array-item identity editor:
//
//	go test -run 'ArrayItem' -v .
//
// The existing path-based editor is covered by the rest of the suite:
//
//	go test -v .

// TestSetArrayItemByID is the first array example. It updates the element
// whose identity field equals "b" and shows the edit is independent of the
// element's position in the array.
func TestSetArrayItemByID(t *testing.T) {
	value := map[string]interface{}{"id": "b", "v": 20}

	// "b" sits at index 1.
	json1 := `{"friends":[{"id":"a","v":1},{"id":"b","v":2}]}`
	res, err := SetArrayItem(json1, "friends", "id", "b", value)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1},{"id":"b","v":20}]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}

	// "b" sits at index 0. Position must not matter.
	json2 := `{"friends":[{"id":"b","v":2},{"id":"a","v":1}]}`
	res, err = SetArrayItem(json2, "friends", "id", "b", value)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"b","v":20},{"id":"a","v":1}]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}
}

// TestDeleteArrayItemByID is the second array example. It deletes the element
// whose identity field equals "b" regardless of its position.
func TestDeleteArrayItemByID(t *testing.T) {
	// "b" sits at index 1.
	json1 := `{"friends":[{"id":"a","v":1},{"id":"b","v":2}]}`
	res, err := DeleteArrayItem(json1, "friends", "id", "b")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1}]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}

	// "b" sits at index 0. Position must not matter.
	json2 := `{"friends":[{"id":"b","v":2},{"id":"a","v":1}]}`
	res, err = DeleteArrayItem(json2, "friends", "id", "b")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1}]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}
}

// TestSetArrayItemBytesPreserved proves that the bytes of every unselected
// element are preserved exactly, because the edit is applied at the original
// byte position of the matched element.
func TestSetArrayItemBytesPreserved(t *testing.T) {
	json := "{\"friends\":[ {\"id\":\"a\",\"v\":1} , {\"id\":\"b\",\"v\":2} ]}"
	res, err := SetArrayItem(json, "friends", "id", "b",
		map[string]interface{}{"id": "b", "v": 20})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"friends\":[ {\"id\":\"a\",\"v\":1} , {\"id\":\"b\",\"v\":20} ]}"
	if res != want {
		t.Fatalf("got %q, want %q", res, want)
	}
	// The unselected element's bytes must be unchanged.
	if !strings.Contains(res, `{"id":"a","v":1}`) {
		t.Fatalf("unselected element bytes were changed: %q", res)
	}
}

// TestSetArrayItemDuplicateRejected proves that two elements sharing the same
// identity are rejected before any edit is made, and that the returned text is
// the original unchanged document.
func TestSetArrayItemDuplicateRejected(t *testing.T) {
	json := `{"friends":[{"id":"b","v":1},{"id":"b","v":2}]}`
	value := map[string]interface{}{"id": "b", "v": 3}

	res, err := SetArrayItem(json, "friends", "id", "b", value)
	if err == nil {
		t.Fatal("expected duplicate identity error")
	}
	if !strings.Contains(err.Error(), "duplicate identity") {
		t.Fatalf("expected duplicate identity error, got %q", err)
	}
	if !strings.Contains(err.Error(), "elements 0 and 1") {
		t.Fatalf("expected positions in error, got %q", err)
	}
	// Failure must produce no partial text: the original bytes are returned.
	if res != json {
		t.Fatalf("expected unchanged text on failure, got %q", res)
	}
}

// TestSetArrayItemMissingID proves that an element missing the identity field
// is reported with its position, and that the returned text is unchanged.
func TestSetArrayItemMissingID(t *testing.T) {
	json := `{"friends":[{"id":"a","v":1},{"v":2}]}`
	value := map[string]interface{}{"id": "a", "v": 3}

	res, err := SetArrayItem(json, "friends", "id", "a", value)
	if err == nil {
		t.Fatal("expected missing identity error")
	}
	if !strings.Contains(err.Error(), "missing identity field") {
		t.Fatalf("expected missing identity error, got %q", err)
	}
	if !strings.Contains(err.Error(), `element 1`) {
		t.Fatalf("expected element position in error, got %q", err)
	}
	if res != json {
		t.Fatalf("expected unchanged text on failure, got %q", res)
	}
}

// TestSetArrayItemNonScalarID proves that a non-scalar identity field is
// rejected with its position.
func TestSetArrayItemNonScalarID(t *testing.T) {
	json := `{"friends":[{"id":{"x":1},"v":1}]}`
	value := map[string]interface{}{"id": "a", "v": 3}

	res, err := SetArrayItem(json, "friends", "id", "a", value)
	if err == nil {
		t.Fatal("expected non-scalar identity error")
	}
	if !strings.Contains(err.Error(), "scalar") {
		t.Fatalf("expected scalar error, got %q", err)
	}
	if res != json {
		t.Fatalf("expected unchanged text on failure, got %q", res)
	}
}

// TestSetArrayItemTypeStrict proves that the string "1" and the number 1 are
// two different identities: a string id only matches string fields, and a
// numeric id only matches number fields.
func TestSetArrayItemTypeStrict(t *testing.T) {
	json := `{"items":[{"id":1,"v":"num"},{"id":"1","v":"str"}]}`

	// Looking for the string "1" must match only the string element.
	res, err := SetArrayItem(json, "items", "id", "1",
		map[string]interface{}{"id": "1", "v": "str-updated"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"items":[{"id":1,"v":"num"},{"id":"1","v":"str-updated"}]}`; res != want {
		t.Fatalf("string id: got %q, want %q", res, want)
	}

	// Looking for the number 1 must match only the number element.
	res, err = SetArrayItem(json, "items", "id", 1,
		map[string]interface{}{"id": 1, "v": "num-updated"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"items":[{"id":1,"v":"num-updated"},{"id":"1","v":"str"}]}`; res != want {
		t.Fatalf("numeric id: got %q, want %q", res, want)
	}
}

// TestSetArrayItemBadIDArgument proves that a non-scalar id argument is
// rejected.
func TestSetArrayItemBadIDArgument(t *testing.T) {
	json := `{"friends":[{"id":"a","v":1}]}`
	value := map[string]interface{}{"id": "a", "v": 3}

	res, err := SetArrayItem(json, "friends", "id", map[string]interface{}{"x": 1}, value)
	if err == nil {
		t.Fatal("expected scalar id argument error")
	}
	if !strings.Contains(err.Error(), "scalar") {
		t.Fatalf("expected scalar error, got %q", err)
	}
	if res != json {
		t.Fatalf("expected unchanged text on failure, got %q", res)
	}
}

// TestSetArrayItemNotFound proves that looking up an unknown identity reports
// an error and leaves the text unchanged.
func TestSetArrayItemNotFound(t *testing.T) {
	json := `{"friends":[{"id":"a","v":1}]}`
	value := map[string]interface{}{"id": "z", "v": 3}

	res, err := SetArrayItem(json, "friends", "id", "z", value)
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "no element with identity") {
		t.Fatalf("expected not-found error, got %q", err)
	}
	if res != json {
		t.Fatalf("expected unchanged text on failure, got %q", res)
	}
}

// TestSetArrayItemRaw sets a premarshalled raw json value.
func TestSetArrayItemRaw(t *testing.T) {
	json := `{"friends":[{"id":"a","v":1}]}`
	res, err := SetArrayItemRaw(json, "friends", "id", "a", `{"id":"a","v":99}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":99}]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}
}

// TestSetArrayItemScalarValue replaces an element with a scalar value.
func TestSetArrayItemScalarValue(t *testing.T) {
	json := `{"friends":[{"id":"a","v":1}]}`
	res, err := SetArrayItem(json, "friends", "id", "a", "replaced")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":["replaced"]}`; res != want {
		t.Fatalf("got %q, want %q", res, want)
	}
}

// TestSetArrayItemBytes exercises the byte-slice entry points directly.
func TestSetArrayItemBytes(t *testing.T) {
	json := []byte(`{"friends":[{"id":"a","v":1},{"id":"b","v":2}]}`)

	// A string value is stringified.
	res, err := SetArrayItemBytes(json, "friends", "id", "b", "replaced")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1},"replaced"]}`; string(res) != want {
		t.Fatalf("got %q, want %q", string(res), want)
	}

	// Raw json is set verbatim.
	res, err = SetArrayItemRawBytes(json, "friends", "id", "b", []byte(`{"id":"b","v":20}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1},{"id":"b","v":20}]}`; string(res) != want {
		t.Fatalf("got %q, want %q", string(res), want)
	}

	res, err = DeleteArrayItemBytes(json, "friends", "id", "b")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"friends":[{"id":"a","v":1}]}`; string(res) != want {
		t.Fatalf("got %q, want %q", string(res), want)
	}
}
