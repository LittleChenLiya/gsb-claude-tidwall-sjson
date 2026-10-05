package sjson

import (
	jsongo "encoding/json"
	"fmt"
	"strconv"
	"unsafe"

	"github.com/tidwall/gjson"
)

// SetArrayItem sets the value of the array element identified by an identity
// field. The path locates the target array, idField is the scalar identity
// field within each element, and id is the identity value to match.
//
// Before any edit is made, the whole target array is scanned: every element
// must expose the identity field as a scalar (string, number or boolean) and
// every identity must be unique. Missing identities, non-scalar identities and
// duplicate identities all produce an error that reports the element position,
// and no partial text is ever returned on failure.
//
// Numbers and strings are never considered equal even when their textual
// representations match: the string "1" and the number 1 are two different
// identities.
//
// The edit is applied at the original byte position of the matched element,
// so the bytes of every unselected element are preserved exactly.
//
// A value of dtype{} (as used by Delete) removes the matched element.
func SetArrayItem(json, path, idField string, id, value interface{}) (string, error) {
	res, err := SetArrayItemBytes([]byte(json), path, idField, id, value)
	return string(res), err
}

// SetArrayItemBytes sets the value of the array element identified by an
// identity field. If working with bytes, this method is preferred over
// SetArrayItemBytes(string(data), path, idField, id, value).
func SetArrayItemBytes(json []byte, path, idField string, id, value interface{}) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	switch v := value.(type) {
	case dtype:
		return setArrayItem(jstr, path, idField, id, "", false, true)
	case string:
		return setArrayItem(jstr, path, idField, id, v, true, false)
	case []byte:
		return setArrayItem(jstr, path, idField, id,
			*(*string)(unsafe.Pointer(&v)), true, false)
	case bool:
		if v {
			return setArrayItem(jstr, path, idField, id, "true", false, false)
		}
		return setArrayItem(jstr, path, idField, id, "false", false, false)
	case int:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatInt(int64(v), 10), false, false)
	case int8:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatInt(int64(v), 10), false, false)
	case int16:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatInt(int64(v), 10), false, false)
	case int32:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatInt(int64(v), 10), false, false)
	case int64:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatInt(int64(v), 10), false, false)
	case uint:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatUint(uint64(v), 10), false, false)
	case uint8:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatUint(uint64(v), 10), false, false)
	case uint16:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatUint(uint64(v), 10), false, false)
	case uint32:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatUint(uint64(v), 10), false, false)
	case uint64:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatUint(uint64(v), 10), false, false)
	case float32:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatFloat(float64(v), 'f', -1, 64), false, false)
	case float64:
		return setArrayItem(jstr, path, idField, id,
			strconv.FormatFloat(v, 'f', -1, 64), false, false)
	default:
		b, merr := jsongo.Marshal(value)
		if merr != nil {
			return nil, merr
		}
		return setArrayItem(jstr, path, idField, id,
			*(*string)(unsafe.Pointer(&b)), false, false)
	}
}

// SetArrayItemRaw sets a raw json value for the array element identified by an
// identity field. This works the same as SetArrayItem except that the value is
// set as a raw block of json, which allows for setting premarshalled json
// objects.
func SetArrayItemRaw(json, path, idField string, id interface{}, value string) (string, error) {
	res, err := SetArrayItemRawBytes([]byte(json), path, idField, id, []byte(value))
	return string(res), err
}

// SetArrayItemRawBytes sets a raw json value for the array element identified
// by an identity field. If working with bytes, this method is preferred over
// SetArrayItemRawBytes(string(data), path, idField, id, value).
func SetArrayItemRawBytes(json []byte, path, idField string, id interface{}, value []byte) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	vstr := *(*string)(unsafe.Pointer(&value))
	return setArrayItem(jstr, path, idField, id, vstr, false, false)
}

// DeleteArrayItem deletes the array element identified by an identity field.
// The whole target array is scanned for unique identities before any deletion
// is performed.
func DeleteArrayItem(json, path, idField string, id interface{}) (string, error) {
	res, err := DeleteArrayItemBytes([]byte(json), path, idField, id)
	return string(res), err
}

// DeleteArrayItemBytes deletes the array element identified by an identity
// field. If working with bytes, this method is preferred over
// DeleteArrayItemBytes(string(data), path, idField, id).
func DeleteArrayItemBytes(json []byte, path, idField string, id interface{}) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	return setArrayItem(jstr, path, idField, id, "", false, true)
}

// arrayIdentityKey builds a type-tagged identity key. The tag makes strings
// and numbers distinct identities even when their textual representations are
// equal.
func arrayIdentityKey(id interface{}) (string, error) {
	switch v := id.(type) {
	case string:
		return "s:" + v, nil
	case bool:
		if v {
			return "b:true", nil
		}
		return "b:false", nil
	case int:
		return "n:" + strconv.FormatInt(int64(v), 10), nil
	case int8:
		return "n:" + strconv.FormatInt(int64(v), 10), nil
	case int16:
		return "n:" + strconv.FormatInt(int64(v), 10), nil
	case int32:
		return "n:" + strconv.FormatInt(int64(v), 10), nil
	case int64:
		return "n:" + strconv.FormatInt(int64(v), 10), nil
	case uint:
		return "n:" + strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return "n:" + strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return "n:" + strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return "n:" + strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return "n:" + strconv.FormatUint(uint64(v), 10), nil
	case float32:
		return "n:" + strconv.FormatFloat(float64(v), 'g', -1, 64), nil
	case float64:
		return "n:" + strconv.FormatFloat(v, 'g', -1, 64), nil
	case jsongo.Number:
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil {
			return "", fmt.Errorf("invalid identity number %q: %s", v.String(), err)
		}
		return "n:" + strconv.FormatFloat(f, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("identity must be a scalar (string, number, or boolean), got %T", id)
	}
}

// arrayIdentityKeyFromResult builds a type-tagged identity key from a gjson
// result. Only scalar results are accepted.
func arrayIdentityKeyFromResult(r gjson.Result) (string, error) {
	switch r.Type {
	case gjson.String:
		return "s:" + r.Str, nil
	case gjson.True:
		return "b:true", nil
	case gjson.False:
		return "b:false", nil
	case gjson.Number:
		return "n:" + strconv.FormatFloat(r.Num, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("identity must be a scalar (string, number, or boolean)")
	}
}

// arrayIdentityLabel describes an identity for error messages.
func arrayIdentityLabel(id interface{}) string {
	switch v := id.(type) {
	case string:
		return fmt.Sprintf("%q (string)", v)
	case bool:
		return fmt.Sprintf("%t (bool)", v)
	default:
		return fmt.Sprintf("%v (%T)", id, id)
	}
}

// setArrayItem is the shared implementation. It first validates the whole
// target array (every element must expose the identity field as a scalar and
// every identity must be unique) and only then edits the matched element at
// its original byte position.
func setArrayItem(jstr, path, idField string, id interface{}, raw string,
	stringify, del bool) ([]byte, error) {
	if path == "" {
		return []byte(jstr), &errorType{"path cannot be empty"}
	}
	if idField == "" {
		return []byte(jstr), &errorType{"id field cannot be empty"}
	}
	want, err := arrayIdentityKey(id)
	if err != nil {
		return []byte(jstr), &errorType{err.Error()}
	}
	arr := gjson.Get(jstr, path)
	if !arr.Exists() {
		return []byte(jstr), &errorType{
			fmt.Sprintf("array at path %q not found", path)}
	}
	if !arr.IsArray() {
		return []byte(jstr), &errorType{
			fmt.Sprintf("value at path %q is not an array", path)}
	}
	if arr.Index <= 0 {
		return []byte(jstr), &errorType{
			fmt.Sprintf("could not determine position of array at path %q", path)}
	}
	elems := arr.Array()
	seen := make(map[string]int, len(elems))
	match := -1
	for i, elem := range elems {
		fv := elem.Get(idField)
		if !fv.Exists() {
			return []byte(jstr), &errorType{fmt.Sprintf(
				"missing identity field %q at element %d of array %q (byte offset %d)",
				idField, i, path, elem.Index)}
		}
		key, ferr := arrayIdentityKeyFromResult(fv)
		if ferr != nil {
			return []byte(jstr), &errorType{fmt.Sprintf(
				"identity field %q at element %d of array %q (byte offset %d): %s",
				idField, i, path, elem.Index, ferr.Error())}
		}
		if prev, ok := seen[key]; ok {
			return []byte(jstr), &errorType{fmt.Sprintf(
				"duplicate identity for field %q at elements %d and %d of array %q (byte offsets %d and %d)",
				idField, prev, i, path, elems[prev].Index, elem.Index)}
		}
		seen[key] = i
		if key == want {
			match = i
		}
	}
	if match < 0 {
		return []byte(jstr), &errorType{fmt.Sprintf(
			"no element with identity %s found in array %q",
			arrayIdentityLabel(id), path)}
	}
	elem := elems[match]
	if del {
		buf := make([]byte, 0, len(jstr))
		buf = append(buf, jstr[:elem.Index]...)
		var delNext bool
		buf, delNext = deleteTailItem(buf)
		exidx := 0
		if delNext {
			pos := elem.Index + len(elem.Raw)
			for k := pos; k < len(jstr); k++ {
				if jstr[k] <= ' ' {
					continue
				}
				if jstr[k] == ',' {
					exidx = k - pos + 1
				}
				break
			}
		}
		buf = append(buf, jstr[elem.Index+len(elem.Raw)+exidx:]...)
		return buf, nil
	}
	buf := make([]byte, 0, len(jstr)-len(elem.Raw)+len(raw)+2)
	buf = append(buf, jstr[:elem.Index]...)
	if stringify {
		buf = appendStringify(buf, raw)
	} else {
		buf = append(buf, raw...)
	}
	buf = append(buf, jstr[elem.Index+len(elem.Raw):]...)
	return buf, nil
}
