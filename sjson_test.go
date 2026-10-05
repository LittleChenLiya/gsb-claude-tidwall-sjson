package sjson

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
)

const (
	setRaw    = 1
	setBool   = 2
	setInt    = 3
	setFloat  = 4
	setString = 5
	setDelete = 6
)

func sortJSON(json string) string {
	opts := pretty.Options{SortKeys: true}
	return string(pretty.Ugly(pretty.PrettyOptions([]byte(json), &opts)))
}

func testRaw(t *testing.T, kind int, expect, json, path string, value interface{}) {
	t.Helper()
	expect = sortJSON(expect)
	var json2 string
	var err error
	switch kind {
	default:
		json2, err = Set(json, path, value)
	case setRaw:
		json2, err = SetRaw(json, path, value.(string))
	case setDelete:
		json2, err = Delete(json, path)
	}

	if err != nil {
		t.Fatal(err)
	}
	json2 = sortJSON(json2)
	if json2 != expect {
		t.Fatalf("expected '%v', got '%v'", expect, json2)
	}
	var json3 []byte
	switch kind {
	default:
		json3, err = SetBytes([]byte(json), path, value)
	case setRaw:
		json3, err = SetRawBytes([]byte(json), path, []byte(value.(string)))
	case setDelete:
		json3, err = DeleteBytes([]byte(json), path)
	}
	json3 = []byte(sortJSON(string(json3)))
	if err != nil {
		t.Fatal(err)
	} else if string(json3) != expect {
		t.Fatalf("expected '%v', got '%v'", expect, string(json3))
	}
}
func TestBasic(t *testing.T) {
	testRaw(t, setRaw, `[{"hiw":"planet","hi":"world"}]`, `[{"hi":"world"}]`, "0.hiw", `"planet"`)
	testRaw(t, setRaw, `[true]`, ``, "0", `true`)
	testRaw(t, setRaw, `[null,true]`, ``, "1", `true`)
	testRaw(t, setRaw, `[1,null,true]`, `[1]`, "2", `true`)
	testRaw(t, setRaw, `[1,true,false]`, `[1,null,false]`, "1", `true`)
	testRaw(t, setRaw,
		`[1,{"hello":"when","this":[0,null,2]},false]`,
		`[1,{"hello":"when","this":[0,1,2]},false]`,
		"1.this.1", `null`)
	testRaw(t, setRaw,
		`{"a":1,"b":{"hello":"when","this":[0,null,2]},"c":false}`,
		`{"a":1,"b":{"hello":"when","this":[0,1,2]},"c":false}`,
		"b.this.1", `null`)
	testRaw(t, setRaw,
		`{"a":1,"b":{"hello":"when","this":[0,null,2,null,4]},"c":false}`,
		`{"a":1,"b":{"hello":"when","this":[0,null,2]},"c":false}`,
		"b.this.4", `4`)
	testRaw(t, setRaw,
		`{"b":{"this":[null,null,null,null,4]}}`,
		``,
		"b.this.4", `4`)
	testRaw(t, setRaw,
		`[null,{"this":[null,null,null,null,4]}]`,
		``,
		"1.this.4", `4`)
	testRaw(t, setRaw,
		`{"1":{"this":[null,null,null,null,4]}}`,
		``,
		":1.this.4", `4`)
	testRaw(t, setRaw,
		`{":1":{"this":[null,null,null,null,4]}}`,
		``,
		"\\:1.this.4", `4`)
	testRaw(t, setRaw,
		`{":\\1":{"this":[null,null,null,null,{".HI":4}]}}`,
		``,
		"\\:\\\\1.this.4.\\.HI", `4`)
	testRaw(t, setRaw,
		`{"app.token":"cde"}`,
		`{"app.token":"abc"}`,
		"app\\.token", `"cde"`)
	testRaw(t, setRaw,
		`{"b":{"this":{"😇":""}}}`,
		``,
		"b.this.😇", `""`)
	testRaw(t, setRaw,
		`[ 1,2  ,3]`,
		`  [ 1,2  ] `,
		"-1", `3`)
	testRaw(t, setInt, `[1234]`, ``, `0`, int64(1234))
	testRaw(t, setFloat, `[1234.5]`, ``, `0`, float64(1234.5))
	testRaw(t, setString, `["1234.5"]`, ``, `0`, "1234.5")
	testRaw(t, setBool, `[true]`, ``, `0`, true)
	testRaw(t, setBool, `[null]`, ``, `0`, nil)
	testRaw(t, setString, `{"arr":[1]}`, ``, `arr.-1`, 1)
	testRaw(t, setString, `{"a":"\\"}`, ``, `a`, "\\")
	testRaw(t, setString, `{"a":"C:\\Windows\\System32"}`, ``, `a`, `C:\Windows\System32`)
}

func TestDelete(t *testing.T) {
	testRaw(t, setDelete, `[456]`, `[123,456]`, `0`, nil)
	testRaw(t, setDelete, `[123,789]`, `[123,456,789]`, `1`, nil)
	testRaw(t, setDelete, `[123,456]`, `[123,456,789]`, `-1`, nil)
	testRaw(t, setDelete, `{"a":[123,456]}`, `{"a":[123,456,789]}`, `a.-1`, nil)
	testRaw(t, setDelete, `{"and":"another"}`, `{"this":"that","and":"another"}`, `this`, nil)
	testRaw(t, setDelete, `{"this":"that"}`, `{"this":"that","and":"another"}`, `and`, nil)
	testRaw(t, setDelete, `{}`, `{"and":"another"}`, `and`, nil)
	testRaw(t, setDelete, `{"1":"2"}`, `{"1":"2"}`, `3`, nil)
}

// TestRandomData is a fuzzing test that throws random data at SetRaw
// function looking for panics.
func TestRandomData(t *testing.T) {
	var lstr string
	defer func() {
		if v := recover(); v != nil {
			println("'" + hex.EncodeToString([]byte(lstr)) + "'")
			println("'" + lstr + "'")
			panic(v)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 200)
	for i := 0; i < 2000000; i++ {
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		lstr = string(b[:n])
		SetRaw(lstr, "zzzz.zzzz.zzzz", "123")
	}
}

func TestDeleteIssue21(t *testing.T) {
	json := `{"country_code_from":"NZ","country_code_to":"SA","date_created":"2018-09-13T02:56:11.25783Z","date_updated":"2018-09-14T03:15:16.67356Z","disabled":false,"last_edited_by":"Developers","id":"a3e...bc454","merchant_id":"f2b...b91abf","signed_date":"2018-02-01T00:00:00Z","start_date":"2018-03-01T00:00:00Z","url":"https://www.google.com"}`
	res1 := gjson.Get(json, "date_updated")
	var err error
	json, err = Delete(json, "date_updated")
	if err != nil {
		t.Fatal(err)
	}
	res2 := gjson.Get(json, "date_updated")
	res3 := gjson.Get(json, "date_created")
	if !res1.Exists() || res2.Exists() || !res3.Exists() {
		t.Fatal("bad news")
	}

	// We change the number of characters in this to make the section of the string before the section that we want to delete a certain length

	//---------------------------
	lenBeforeToDeleteIs307AsBytes := `{"1":"","0":"012345678901234567890123456789012345678901234567890123456789012345678901234567","to_delete":"0","2":""}`

	expectedForLenBefore307AsBytes := `{"1":"","0":"012345678901234567890123456789012345678901234567890123456789012345678901234567","2":""}`
	//---------------------------

	//---------------------------
	lenBeforeToDeleteIs308AsBytes := `{"1":"","0":"0123456789012345678901234567890123456789012345678901234567890123456789012345678","to_delete":"0","2":""}`

	expectedForLenBefore308AsBytes := `{"1":"","0":"0123456789012345678901234567890123456789012345678901234567890123456789012345678","2":""}`
	//---------------------------

	//---------------------------
	lenBeforeToDeleteIs309AsBytes := `{"1":"","0":"01234567890123456789012345678901234567890123456789012345678901234567890123456","to_delete":"0","2":""}`

	expectedForLenBefore309AsBytes := `{"1":"","0":"01234567890123456789012345678901234567890123456789012345678901234567890123456","2":""}`
	//---------------------------

	var data = []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "len before \"to_delete\"... = 307",
			input:    lenBeforeToDeleteIs307AsBytes,
			expected: expectedForLenBefore307AsBytes,
		},
		{
			desc:     "len before \"to_delete\"... = 308",
			input:    lenBeforeToDeleteIs308AsBytes,
			expected: expectedForLenBefore308AsBytes,
		},
		{
			desc:     "len before \"to_delete\"... = 309",
			input:    lenBeforeToDeleteIs309AsBytes,
			expected: expectedForLenBefore309AsBytes,
		},
	}

	for i, d := range data {
		result, err := Delete(d.input, "to_delete")

		if err != nil {
			t.Error(fmtErrorf(testError{
				unexpected: "error",
				desc:       d.desc,
				i:          i,
				lenInput:   len(d.input),
				input:      d.input,
				expected:   d.expected,
				result:     result,
			}))
		}
		if result != d.expected {
			t.Error(fmtErrorf(testError{
				unexpected: "result",
				desc:       d.desc,
				i:          i,
				lenInput:   len(d.input),
				input:      d.input,
				expected:   d.expected,
				result:     result,
			}))
		}
	}
}

type testError struct {
	unexpected string
	desc       string
	i          int
	lenInput   int
	input      interface{}
	expected   interface{}
	result     interface{}
}

func fmtErrorf(e testError) string {
	return fmt.Sprintf(
		"Unexpected %s:\n\t"+
			"for=%q\n\t"+
			"i=%d\n\t"+
			"len(input)=%d\n\t"+
			"input=%v\n\t"+
			"expected=%v\n\t"+
			"result=%v",
		e.unexpected, e.desc, e.i, e.lenInput, e.input, e.expected, e.result,
	)
}

func TestSetDotKeyIssue10(t *testing.T) {
	json := `{"app.token":"abc"}`
	json, _ = Set(json, `app\.token`, "cde")
	if json != `{"app.token":"cde"}` {
		t.Fatalf("expected '%v', got '%v'", `{"app.token":"cde"}`, json)
	}
}
func TestDeleteDotKeyIssue19(t *testing.T) {
	json := []byte(`{"data":{"key1":"value1","key2.something":"value2"}}`)
	json, _ = DeleteBytes(json, `data.key2\.something`)
	if string(json) != `{"data":{"key1":"value1"}}` {
		t.Fatalf("expected '%v', got '%v'", `{"data":{"key1":"value1"}}`, json)
	}
}

func TestIssue36(t *testing.T) {
	var json = `
	{
	    "size": 1000
    }
`
	var raw = `
	{
	    "sample": "hello"
	}
`
	_ = raw
	if true {
		json, _ = SetRaw(json, "aggs", raw)
	}
	if !gjson.Valid(json) {
		t.Fatal("invalid json")
	}
	res := gjson.Get(json, "aggs.sample").String()
	if res != "hello" {
		t.Fatal("unexpected result")
	}
}

var example = `
{
	"name": {"first": "Tom", "last": "Anderson"},
	"age":37,
	"children": ["Sara","Alex","Jack"],
	"fav.movie": "Deer Hunter",
	"friends": [
	  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
	  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
	  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
	]
  }
  `

func TestIndex(t *testing.T) {
	path := `friends.#(last="Murphy").last`
	json, err := Set(example, path, "Johnson")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, "friends.#.last").String() != `["Johnson","Craig","Murphy"]` {
		t.Fatal("mismatch")
	}
}

func TestIndexes(t *testing.T) {
	path := `friends.#(last="Murphy")#.last`
	json, err := Set(example, path, "Johnson")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, "friends.#.last").String() != `["Johnson","Craig","Johnson"]` {
		t.Fatal("mismatch")
	}
}

func TestIssue61(t *testing.T) {
	json := `{
		"@context": {
		  "rdfs": "http://www.w3.org/2000/01/rdf-schema#",
		  "@vocab": "http://schema.org/",
		  "sh": "http://www.w3.org/ns/shacl#"
		}
	}`
	json1, _ := Set(json, "@context.@vocab", "newval")
	if gjson.Get(json1, "@context.@vocab").String() != "newval" {
		t.Fail()
	}
}

// TestConditional covers the explicit conditional Set/Delete behavior.
func TestConditional(t *testing.T) {
	// CondMissing: a missing value can be set, but an existing null value
	// must be rejected because the key is present.
	if out, err := SetIf(`{}`, "a", 1, CondMissing); err != nil || out != `{"a":1}` {
		t.Fatalf("missing set: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":null}`, "a", 1, CondMissing); err != nil || out != `{"a":null}` {
		t.Fatalf("missing+null rejected: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":1}`, "a", 2, CondMissing); err != nil || out != `{"a":1}` {
		t.Fatalf("missing+number rejected: got %q err=%v", out, err)
	}

	// CondNull: only an existing null value can be replaced.
	if out, err := SetIf(`{"a":null}`, "a", 1, CondNull); err != nil || out != `{"a":1}` {
		t.Fatalf("null set: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":1}`, "a", 2, CondNull); err != nil || out != `{"a":1}` {
		t.Fatalf("null+number rejected: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{}`, "a", 1, CondNull); err != nil || out != `{}` {
		t.Fatalf("null+missing rejected: got %q err=%v", out, err)
	}

	// CondString: only an existing string value can be replaced.
	if out, err := SetIf(`{"a":"x"}`, "a", "y", CondString); err != nil || out != `{"a":"y"}` {
		t.Fatalf("string set: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":1}`, "a", "y", CondString); err != nil || out != `{"a":1}` {
		t.Fatalf("string+number rejected: got %q err=%v", out, err)
	}

	// CondNumber: only an existing number value can be replaced.
	if out, err := SetIf(`{"a":1}`, "a", 2, CondNumber); err != nil || out != `{"a":2}` {
		t.Fatalf("number set: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":"x"}`, "a", 2, CondNumber); err != nil || out != `{"a":"x"}` {
		t.Fatalf("number+string rejected: got %q err=%v", out, err)
	}

	// CondBool: only an existing boolean value can be replaced.
	if out, err := SetIf(`{"a":true}`, "a", false, CondBool); err != nil || out != `{"a":false}` {
		t.Fatalf("bool set: got %q err=%v", out, err)
	}
	if out, err := SetIf(`{"a":1}`, "a", false, CondBool); err != nil || out != `{"a":1}` {
		t.Fatalf("bool+number rejected: got %q err=%v", out, err)
	}

	// CondObject and CondArray.
	if out, err := SetRawIf(`{"a":{"b":1}}`, "a", `{"c":2}`, CondObject); err != nil || out != `{"a":{"c":2}}` {
		t.Fatalf("object set: got %q err=%v", out, err)
	}
	if out, err := SetRawIf(`{"a":[1,2]}`, "a", `[3,4]`, CondArray); err != nil || out != `{"a":[3,4]}` {
		t.Fatalf("array set: got %q err=%v", out, err)
	}
	if out, err := SetRawIf(`{"a":1}`, "a", `{}`, CondObject); err != nil || out != `{"a":1}` {
		t.Fatalf("object+number rejected: got %q err=%v", out, err)
	}

	// CondNone behaves like the ordinary Set.
	if out, err := SetIf(`{"a":1}`, "a", 2, CondNone); err != nil || out != `{"a":2}` {
		t.Fatalf("none set: got %q err=%v", out, err)
	}
}

// TestConditionalObjectKeyNotIndex verifies that an object numeric key is
// never treated as an array index when checking the condition.
func TestConditionalObjectKeyNotIndex(t *testing.T) {
	// The object key "0" exists, so CondMissing must be rejected.
	if out, err := SetIf(`{"0":"x"}`, "0", "y", CondMissing); err != nil || out != `{"0":"x"}` {
		t.Fatalf("object key 0 + missing rejected: got %q err=%v", out, err)
	}
	// The object key "0" is a string, so CondString allows the update.
	if out, err := SetIf(`{"0":"x"}`, "0", "y", CondString); err != nil || out != `{"0":"y"}` {
		t.Fatalf("object key 0 + string: got %q err=%v", out, err)
	}
	// A nested object key "0" is also not an array index.
	if out, err := SetIf(`{"a":{"0":"x"}}`, "a.0", "y", CondString); err != nil || out != `{"a":{"0":"y"}}` {
		t.Fatalf("nested object key 0 + string: got %q err=%v", out, err)
	}
	// An array index "0" is still an array index.
	if out, err := SetIf(`["x"]`, "0", "y", CondString); err != nil || out != `["y"]` {
		t.Fatalf("array index 0 + string: got %q err=%v", out, err)
	}
	// A nested array index "0" is still an array index.
	if out, err := SetIf(`{"a":["x"]}`, "a.0", "y", CondString); err != nil || out != `{"a":["y"]}` {
		t.Fatalf("nested array index 0 + string: got %q err=%v", out, err)
	}
}

// TestConditionalDelete verifies conditional Delete behavior, including
// checking the parent type when deleting a nested value.
func TestConditionalDelete(t *testing.T) {
	// Delete only when the value is null.
	if out, err := DeleteIf(`{"a":null,"b":1}`, "a", CondNull); err != nil || out != `{"b":1}` {
		t.Fatalf("delete null: got %q err=%v", out, err)
	}
	// A non-null value is not deleted.
	if out, err := DeleteIf(`{"a":1,"b":1}`, "a", CondNull); err != nil || out != `{"a":1,"b":1}` {
		t.Fatalf("delete non-null rejected: got %q err=%v", out, err)
	}
	// CondMissing on an existing value is rejected.
	if out, err := DeleteIf(`{"a":1}`, "a", CondMissing); err != nil || out != `{"a":1}` {
		t.Fatalf("delete missing+existing rejected: got %q err=%v", out, err)
	}
	// CondMissing on a missing value is a no-op.
	if out, err := DeleteIf(`{"a":1}`, "b", CondMissing); err != nil || out != `{"a":1}` {
		t.Fatalf("delete missing+missing: got %q err=%v", out, err)
	}
	// Deleting a nested value checks the parent type: "a" is an object,
	// so "b" is a key, not an array index.
	if out, err := DeleteIf(`{"a":{"b":null}}`, "a.b", CondNull); err != nil || out != `{"a":{}}` {
		t.Fatalf("delete nested null: got %q err=%v", out, err)
	}
	// Deleting an array element by index.
	if out, err := DeleteIf(`{"a":[1,null,3]}`, "a.1", CondNull); err != nil || out != `{"a":[1,3]}` {
		t.Fatalf("delete array null: got %q err=%v", out, err)
	}
	// A string key into an array parent is not deleted (parent type mismatch).
	if out, err := DeleteIf(`{"a":[1,2,3]}`, "a.b", CondMissing); err != nil || out != `{"a":[1,2,3]}` {
		t.Fatalf("delete array string key: got %q err=%v", out, err)
	}
}

// TestConditionalRawValueRejected verifies that a malformed raw value is
// rejected before the condition is checked.
func TestConditionalRawValueRejected(t *testing.T) {
	if out, err := SetRawIf(`{"a":1}`, "a", `{bad`, CondString); err == nil || out != `{"a":1}` {
		t.Fatalf("invalid raw rejected: got %q err=%v", out, err)
	}
	if out, err := SetRawBytesIf([]byte(`{"a":1}`), "a", []byte(`[1,`), CondString); err == nil || string(out) != `{"a":1}` {
		t.Fatalf("invalid raw bytes rejected: got %q err=%v", out, err)
	}
	// A valid raw value with a mismatched condition leaves bytes unchanged.
	if out, err := SetRawIf(`{"a":1}`, "a", `"x"`, CondString); err != nil || out != `{"a":1}` {
		t.Fatalf("valid raw mismatch: got %q err=%v", out, err)
	}
}

// TestConditionalBytesVariants verifies the byte-slice conditional variants.
func TestConditionalBytesVariants(t *testing.T) {
	if out, err := SetBytesIf([]byte(`{"a":"x"}`), "a", "y", CondString); err != nil || string(out) != `{"a":"y"}` {
		t.Fatalf("SetBytesIf: got %q err=%v", out, err)
	}
	if out, err := DeleteBytesIf([]byte(`{"a":null}`), "a", CondNull); err != nil || string(out) != `{}` {
		t.Fatalf("DeleteBytesIf: got %q err=%v", out, err)
	}
	if out, err := SetRawBytesIf([]byte(`{"a":1}`), "a", []byte(`2`), CondNumber); err != nil || string(out) != `{"a":2}` {
		t.Fatalf("SetRawBytesIf: got %q err=%v", out, err)
	}
}

// TestConditionalThreeUpdates demonstrates three conditional updates applied
// in sequence: a missing set, a string update, and a null delete.
func TestConditionalThreeUpdates(t *testing.T) {
	json := `{"name":"Tom","age":37}`
	var err error
	// 1. Set "name" only if it is a string.
	json, err = SetIf(json, "name", "Anderson", CondString)
	if err != nil {
		t.Fatal(err)
	}
	// 2. Set "age" only if it is a number.
	json, err = SetIf(json, "age", 38, CondNumber)
	if err != nil {
		t.Fatal(err)
	}
	// 3. Delete "extra" only if it is null (it is missing, so no-op).
	json, err = DeleteIf(json, "extra", CondNull)
	if err != nil {
		t.Fatal(err)
	}
	if json != `{"name":"Anderson","age":38}` {
		t.Fatalf("three updates: got %q", json)
	}
}
