package sjson

import (
	"strings"
	"testing"
)

// two-item array used by the identity editing examples
const byidUsers = `{"users":[{"id":"a","v":1},{"id":"b","v":2}],"n":7}`

func TestSetByID(t *testing.T) {
	// update the element whose identity is "b"; it is the second item
	got, err := SetByID(byidUsers, "users", "id", "b",
		map[string]interface{}{"id": "b", "v": 9})
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users":[{"id":"a","v":1},{"id":"b","v":9}],"n":7}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}

func TestSetByIDPositionIndependent(t *testing.T) {
	// the same update by "b" when "b" is the first item
	json := `{"users":[{"id":"b","v":2},{"id":"a","v":1}]}`
	got, err := SetByID(json, "users", "id", "b",
		map[string]interface{}{"id": "b", "v": 9})
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users":[{"id":"b","v":9},{"id":"a","v":1}]}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}

func TestSetByIDDuplicateRejected(t *testing.T) {
	// two elements carry identity "b": the whole update is rejected,
	// even though the target is the unique "a"
	json := `{"users":[{"id":"a","v":1},{"id":"b","v":2},{"id":"b","v":3}]}`
	got, err := SetByID(json, "users", "id", "a", 9)
	if err == nil {
		t.Fatal("expected a duplicate identity error")
	}
	if !strings.Contains(err.Error(), "duplicate identity") ||
		!strings.Contains(err.Error(), "1") ||
		!strings.Contains(err.Error(), "2") {
		t.Fatalf("error should report duplicate positions, got %v", err)
	}
	if got != json {
		t.Fatalf("failure must not produce partial text, got %v", got)
	}
}

func TestSetByIDStringNumberDistinct(t *testing.T) {
	// the string "1" and the number 1 are distinct identities and may
	// coexist in the same array without being duplicates
	json := `{"users":[{"id":1,"v":"num"},{"id":"1","v":"str"}]}`
	got, err := SetByID(json, "users", "id", 1,
		map[string]interface{}{"id": 1, "v": "x"})
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users":[{"id":1,"v":"x"},{"id":"1","v":"str"}]}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
	got, err = SetByID(json, "users", "id", "1",
		map[string]interface{}{"id": "1", "v": "y"})
	if err != nil {
		t.Fatal(err)
	}
	expect = `{"users":[{"id":1,"v":"num"},{"id":"1","v":"y"}]}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}

func TestSetByIDMissingIdentity(t *testing.T) {
	json := `{"users":[{"id":"a","v":1},{"v":2}]}`
	got, err := SetByID(json, "users", "id", "a", 9)
	if err == nil {
		t.Fatal("expected a missing identity error")
	}
	if !strings.Contains(err.Error(), "missing scalar identity field") ||
		!strings.Contains(err.Error(), "index 1") {
		t.Fatalf("error should report the missing position, got %v", err)
	}
	if got != json {
		t.Fatalf("failure must not produce partial text, got %v", got)
	}
}

func TestSetByIDNotFound(t *testing.T) {
	got, err := SetByID(byidUsers, "users", "id", "zzz", 9)
	if err == nil {
		t.Fatal("expected a not found error")
	}
	if !strings.Contains(err.Error(), `no array element with identity`) {
		t.Fatalf("unexpected error %v", err)
	}
	if got != byidUsers {
		t.Fatalf("failure must not produce partial text, got %v", got)
	}
}

func TestSetByIDBadIdentityType(t *testing.T) {
	got, err := SetByID(byidUsers, "users", "id", []string{"a"}, 9)
	if err == nil {
		t.Fatal("expected an identity type error")
	}
	if got != byidUsers {
		t.Fatalf("failure must not produce partial text, got %v", got)
	}
}

func TestSetByIDUnselectedBytesUnchanged(t *testing.T) {
	// only the matched element's bytes are replaced; the surrounding
	// text, including whitespace and the unselected item, is untouched
	json := `{"users": [ {"id":"a","v":1} ,  {"id":"b","v":2} ] , "n": 7}`
	got, err := SetByID(json, "users", "id", "b",
		map[string]interface{}{"id": "b", "v": 9})
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users": [ {"id":"a","v":1} ,  {"id":"b","v":9} ] , "n": 7}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}

func TestDeleteByID(t *testing.T) {
	// delete the second of the two items
	got, err := DeleteByID(byidUsers, "users", "id", "b")
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users":[{"id":"a","v":1}],"n":7}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
	// delete the first of the two items
	got, err = DeleteByID(byidUsers, "users", "id", "a")
	if err != nil {
		t.Fatal(err)
	}
	expect = `{"users":[{"id":"b","v":2}],"n":7}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}

func TestDeleteByIDDuplicateRejected(t *testing.T) {
	json := `[{"id":"b"},{"id":"b"}]`
	got, err := DeleteByID(json, "", "id", "b")
	if err == nil {
		t.Fatal("expected a duplicate identity error")
	}
	if !strings.Contains(err.Error(), "duplicate identity") {
		t.Fatalf("unexpected error %v", err)
	}
	if got != json {
		t.Fatalf("failure must not produce partial text, got %v", got)
	}
}

func TestDeleteByIDBoolIdentity(t *testing.T) {
	json := `{"flags":[{"id":true,"v":1},{"id":false,"v":2}]}`
	got, err := DeleteByID(json, "flags", "id", true)
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"flags":[{"id":false,"v":2}]}`
	if got != expect {
		t.Fatalf("expect %v, got %v", expect, got)
	}
}
