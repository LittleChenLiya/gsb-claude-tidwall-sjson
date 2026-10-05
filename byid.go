package sjson

import (
	jsongo "encoding/json"
	"strconv"
	"unsafe"

	"github.com/tidwall/gjson"
)

// identity is a typed scalar value used to match array elements.
// Identity comparison is type-strict: the JSON number 1 and the JSON
// string "1" are distinct identities and are never auto-equated.
type identity struct {
	typ byte // 's' string, 'n' number, 'b' bool
	str string
	num float64
	b   bool
}

func (id identity) String() string {
	switch id.typ {
	case 's':
		return strconv.Quote(id.str)
	case 'n':
		return strconv.FormatFloat(id.num, 'f', -1, 64)
	default:
		return strconv.FormatBool(id.b)
	}
}

// identityFromResult extracts a scalar identity from a json value.
// It reports false for null, object, and array values.
func identityFromResult(res gjson.Result) (identity, bool) {
	switch res.Type {
	case gjson.String:
		return identity{typ: 's', str: res.Str}, true
	case gjson.Number:
		return identity{typ: 'n', num: res.Num}, true
	case gjson.True:
		return identity{typ: 'b', b: true}, true
	case gjson.False:
		return identity{typ: 'b'}, true
	}
	return identity{}, false
}

var errIdentityType = &errorType{
	"identity must be a string, number, or bool"}

// identityFromValue converts a declared Go scalar into an identity.
// Only string, bool, and the numeric types are accepted.
func identityFromValue(value interface{}) (identity, error) {
	switch v := value.(type) {
	case string:
		return identity{typ: 's', str: v}, nil
	case bool:
		return identity{typ: 'b', b: v}, nil
	case int:
		return identity{typ: 'n', num: float64(v)}, nil
	case int8:
		return identity{typ: 'n', num: float64(v)}, nil
	case int16:
		return identity{typ: 'n', num: float64(v)}, nil
	case int32:
		return identity{typ: 'n', num: float64(v)}, nil
	case int64:
		return identity{typ: 'n', num: float64(v)}, nil
	case uint:
		return identity{typ: 'n', num: float64(v)}, nil
	case uint8:
		return identity{typ: 'n', num: float64(v)}, nil
	case uint16:
		return identity{typ: 'n', num: float64(v)}, nil
	case uint32:
		return identity{typ: 'n', num: float64(v)}, nil
	case uint64:
		return identity{typ: 'n', num: float64(v)}, nil
	case float32:
		return identity{typ: 'n', num: float64(v)}, nil
	case float64:
		return identity{typ: 'n', num: v}, nil
	}
	return identity{}, errIdentityType
}

// rawValue converts a value into its raw json representation, mirroring
// the behavior of SetBytesOptions.
func rawValue(value interface{}) (raw string, stringify bool, err error) {
	switch v := value.(type) {
	default:
		b, merr := jsongo.Marshal(value)
		if merr != nil {
			return "", false, merr
		}
		return *(*string)(unsafe.Pointer(&b)), false, nil
	case string:
		return v, true, nil
	case []byte:
		return *(*string)(unsafe.Pointer(&v)), true, nil
	case bool:
		if v {
			return "true", false, nil
		}
		return "false", false, nil
	case int:
		return strconv.FormatInt(int64(v), 10), false, nil
	case int8:
		return strconv.FormatInt(int64(v), 10), false, nil
	case int16:
		return strconv.FormatInt(int64(v), 10), false, nil
	case int32:
		return strconv.FormatInt(int64(v), 10), false, nil
	case int64:
		return strconv.FormatInt(v, 10), false, nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), false, nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), false, nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), false, nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), false, nil
	case uint64:
		return strconv.FormatUint(v, 10), false, nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64), false, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), false, nil
	}
}

// editByID replaces or deletes the single element of the array at path
// whose identity field equals id.
//
// The entire target array is validated before any edit is made: every
// element must carry a scalar identity at field and all identities must
// be unique. When validation or matching fails, the original json is
// returned along with the error and no partial output is produced.
// Editing splices the original text at the element's byte offsets, so
// the bytes of all unselected elements are preserved verbatim.
func editByID(jstr, path, field string, id identity, raw string,
	stringify, del bool) ([]byte, error) {
	var arr gjson.Result
	if path == "" {
		arr = gjson.Parse(jstr)
	} else {
		arr = gjson.Get(jstr, path)
	}
	if !arr.Exists() {
		return []byte(jstr),
			&errorType{"no value at path \"" + path + "\""}
	}
	if !arr.IsArray() {
		return []byte(jstr),
			&errorType{"value at path \"" + path + "\" is not an array"}
	}
	count := int(arr.Get("#").Int())
	type item struct {
		id  identity
		pos int // byte offset of the element in the original json
		end int // byte offset just past the element
	}
	items := make([]item, 0, count)
	seen := make(map[identity]int, count)
	for i := 0; i < count; i++ {
		el := arr.Get(strconv.Itoa(i))
		eid, ok := identityFromResult(el.Get(field))
		if !ok {
			return []byte(jstr), &errorType{
				"missing scalar identity field \"" + field +
					"\" at array index " + strconv.Itoa(i)}
		}
		if prev, dup := seen[eid]; dup {
			return []byte(jstr), &errorType{
				"duplicate identity " + eid.String() +
					" at array indexes " + strconv.Itoa(prev) +
					" and " + strconv.Itoa(i)}
		}
		seen[eid] = i
		// gjson's Result.Get adds the parent offset, so el.Index is
		// already the element's byte offset in the original json.
		items = append(items, item{
			id:  eid,
			pos: el.Index,
			end: el.Index + len(el.Raw),
		})
	}
	target := -1
	for i := range items {
		if items[i].id == id {
			target = i
			break
		}
	}
	if target < 0 {
		return []byte(jstr), &errorType{
			"no array element with identity " + id.String()}
	}
	it := items[target]
	if del {
		buf := []byte(jstr[:it.pos])
		var exidx int
		var delNextComma bool
		buf, delNextComma = deleteTailItem(buf)
		if delNextComma {
			i, j := it.end, 0
			for ; i < len(jstr); i, j = i+1, j+1 {
				if jstr[i] <= ' ' {
					continue
				}
				if jstr[i] == ',' {
					exidx = j + 1
				}
				break
			}
		}
		buf = append(buf, jstr[it.end+exidx:]...)
		return buf, nil
	}
	buf := make([]byte, 0, len(jstr)-(it.end-it.pos)+len(raw)+2)
	buf = append(buf, jstr[:it.pos]...)
	if stringify {
		buf = appendStringify(buf, raw)
	} else {
		buf = append(buf, raw...)
	}
	buf = append(buf, jstr[it.end:]...)
	return buf, nil
}

// SetByID replaces the element of the array at path whose identity field
// equals id with value. The path is a dot-syntax path to the array, such
// as "users"; an empty path targets the root array. The field is the key
// of the identity inside each element, such as "id".
//
// The whole array is checked first: every element must have a scalar
// (string, number, or bool) identity and all identities must be unique,
// otherwise an error reporting the array index positions is returned and
// the json is left unchanged. Identity matching is type-strict, so the
// string "1" and the number 1 are different identities. Only the matched
// element's bytes are replaced; all other bytes are preserved as-is.
func SetByID(json, path, field string, id, value interface{}) (string, error) {
	jsonh := *(*stringHeader)(unsafe.Pointer(&json))
	jsonbh := sliceHeader{data: jsonh.data, len: jsonh.len, cap: jsonh.len}
	jsonb := *(*[]byte)(unsafe.Pointer(&jsonbh))
	res, err := SetBytesByID(jsonb, path, field, id, value)
	return string(res), err
}

// SetBytesByID replaces an array element by identity, like SetByID.
// If working with bytes, this method is preferred over
// SetByID(string(data), path, field, id, value).
func SetBytesByID(json []byte, path, field string,
	id, value interface{}) ([]byte, error) {
	iid, err := identityFromValue(id)
	if err != nil {
		return json, err
	}
	raw, stringify, err := rawValue(value)
	if err != nil {
		return nil, err
	}
	jstr := *(*string)(unsafe.Pointer(&json))
	return editByID(jstr, path, field, iid, raw, stringify, false)
}

// DeleteByID deletes the element of the array at path whose identity
// field equals id. It follows the same rules as SetByID: the whole
// array's identities are validated for presence and uniqueness first,
// and a failure returns the original json without any partial edit.
func DeleteByID(json, path, field string, id interface{}) (string, error) {
	jsonh := *(*stringHeader)(unsafe.Pointer(&json))
	jsonbh := sliceHeader{data: jsonh.data, len: jsonh.len, cap: jsonh.len}
	jsonb := *(*[]byte)(unsafe.Pointer(&jsonbh))
	res, err := DeleteBytesByID(jsonb, path, field, id)
	return string(res), err
}

// DeleteBytesByID deletes an array element by identity, like DeleteByID.
// If working with bytes, this method is preferred over
// DeleteByID(string(data), path, field, id).
func DeleteBytesByID(json []byte, path, field string,
	id interface{}) ([]byte, error) {
	iid, err := identityFromValue(id)
	if err != nil {
		return json, err
	}
	jstr := *(*string)(unsafe.Pointer(&json))
	return editByID(jstr, path, field, iid, "", false, true)
}
