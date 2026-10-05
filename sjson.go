// Package sjson provides setting json values.
package sjson

import (
	jsongo "encoding/json"
	"sort"
	"strconv"
	"unsafe"

	"github.com/tidwall/gjson"
)

type errorType struct {
	msg string
}

func (err *errorType) Error() string {
	return err.msg
}

// Options represents additional options for the Set and Delete functions.
type Options struct {
	// Optimistic is a hint that the value likely exists which
	// allows for the sjson to perform a fast-track search and replace.
	Optimistic bool
	// ReplaceInPlace is a hint to replace the input json rather than
	// allocate a new json byte slice. When this field is specified
	// the input json will not longer be valid and it should not be used
	// In the case when the destination slice doesn't have enough free
	// bytes to replace the data in place, a new bytes slice will be
	// created under the hood.
	// The Optimistic flag must be set to true and the input must be a
	// byte slice in order to use this field.
	ReplaceInPlace bool
}

// Cond is a precondition on the existing value at the target path for the
// conditional SetIf/DeleteIf family of functions. The condition is checked
// at the real target location during the same parse/traversal pass that
// performs the update: object keys (including numeric-looking keys such as
// "0") and array indexes are distinguished by the actual type of the
// container being traversed. When the condition is not met, the input json
// is returned unchanged and no bytes are modified.
type Cond int

const (
	// CondAny imposes no precondition. The update always proceeds.
	CondAny Cond = iota
	// CondMissing requires that no value exists at the path. An existing
	// null value counts as existing and rejects the update.
	CondMissing
	// CondExists requires that a value of any type exists at the path.
	CondExists
	// CondNull requires the existing value to be json null.
	CondNull
	// CondBool requires the existing value to be true or false.
	CondBool
	// CondNumber requires the existing value to be a number.
	CondNumber
	// CondString requires the existing value to be a string.
	CondString
	// CondObject requires the existing value to be a json object.
	CondObject
	// CondArray requires the existing value to be a json array.
	CondArray
	// CondJSON requires the existing value to be a json object or array.
	CondJSON
)

// condSatisfied reports whether the condition cond holds for the value res
// found at the target path. exists is false when the path does not exist.
func condSatisfied(cond Cond, res gjson.Result, exists bool) bool {
	switch cond {
	case CondAny:
		return true
	case CondMissing:
		return !exists
	case CondExists:
		return exists
	}
	if !exists {
		return false
	}
	switch cond {
	case CondNull:
		return res.Type == gjson.Null
	case CondBool:
		return res.Type == gjson.True || res.Type == gjson.False
	case CondNumber:
		return res.Type == gjson.Number
	case CondString:
		return res.Type == gjson.String
	case CondObject:
		return res.IsObject()
	case CondArray:
		return res.IsArray()
	case CondJSON:
		return res.Type == gjson.JSON
	}
	return false
}

type pathResult struct {
	part  string // current key part
	gpart string // gjson get part
	path  string // remaining path
	force bool   // force a string key
	more  bool   // there is more path to parse
}

func isSimpleChar(ch byte) bool {
	switch ch {
	case '|', '#', '@', '*', '?':
		return false
	default:
		return true
	}
}

func parsePath(path string) (res pathResult, simple bool) {
	var r pathResult
	if len(path) > 0 && path[0] == ':' {
		r.force = true
		path = path[1:]
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			r.part = path[:i]
			r.gpart = path[:i]
			r.path = path[i+1:]
			r.more = true
			return r, true
		}
		if !isSimpleChar(path[i]) {
			return r, false
		}
		if path[i] == '\\' {
			// go into escape mode. this is a slower path that
			// strips off the escape character from the part.
			epart := []byte(path[:i])
			gpart := []byte(path[:i+1])
			i++
			if i < len(path) {
				epart = append(epart, path[i])
				gpart = append(gpart, path[i])
				i++
				for ; i < len(path); i++ {
					if path[i] == '\\' {
						gpart = append(gpart, '\\')
						i++
						if i < len(path) {
							epart = append(epart, path[i])
							gpart = append(gpart, path[i])
						}
						continue
					} else if path[i] == '.' {
						r.part = string(epart)
						r.gpart = string(gpart)
						r.path = path[i+1:]
						r.more = true
						return r, true
					} else if !isSimpleChar(path[i]) {
						return r, false
					}
					epart = append(epart, path[i])
					gpart = append(gpart, path[i])
				}
			}
			// append the last part
			r.part = string(epart)
			r.gpart = string(gpart)
			return r, true
		}
	}
	r.part = path
	r.gpart = path
	return r, true
}

func mustMarshalString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > 0x7f || s[i] == '"' || s[i] == '\\' {
			return true
		}
	}
	return false
}

// appendStringify makes a json string and appends to buf.
func appendStringify(buf []byte, s string) []byte {
	if mustMarshalString(s) {
		b, _ := jsongo.Marshal(s)
		return append(buf, b...)
	}
	buf = append(buf, '"')
	buf = append(buf, s...)
	buf = append(buf, '"')
	return buf
}

// appendBuild builds a json block from a json path.
func appendBuild(buf []byte, array bool, paths []pathResult, raw string,
	stringify bool) []byte {
	if !array {
		buf = appendStringify(buf, paths[0].part)
		buf = append(buf, ':')
	}
	if len(paths) > 1 {
		n, numeric := atoui(paths[1])
		if numeric || (!paths[1].force && paths[1].part == "-1") {
			buf = append(buf, '[')
			buf = appendRepeat(buf, "null,", n)
			buf = appendBuild(buf, true, paths[1:], raw, stringify)
			buf = append(buf, ']')
		} else {
			buf = append(buf, '{')
			buf = appendBuild(buf, false, paths[1:], raw, stringify)
			buf = append(buf, '}')
		}
	} else {
		if stringify {
			buf = appendStringify(buf, raw)
		} else {
			buf = append(buf, raw...)
		}
	}
	return buf
}

// atoui does a rip conversion of string -> unigned int.
func atoui(r pathResult) (n int, ok bool) {
	if r.force {
		return 0, false
	}
	for i := 0; i < len(r.part); i++ {
		if r.part[i] < '0' || r.part[i] > '9' {
			return 0, false
		}
		n = n*10 + int(r.part[i]-'0')
	}
	return n, true
}

// appendRepeat repeats string "n" times and appends to buf.
func appendRepeat(buf []byte, s string, n int) []byte {
	for i := 0; i < n; i++ {
		buf = append(buf, s...)
	}
	return buf
}

// trim does a rip trim
func trim(s string) string {
	for len(s) > 0 {
		if s[0] <= ' ' {
			s = s[1:]
			continue
		}
		break
	}
	for len(s) > 0 {
		if s[len(s)-1] <= ' ' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

// deleteTailItem deletes the previous key or comma.
func deleteTailItem(buf []byte) ([]byte, bool) {
loop:
	for i := len(buf) - 1; i >= 0; i-- {
		// look for either a ',',':','['
		switch buf[i] {
		case '[':
			return buf, true
		case ',':
			return buf[:i], false
		case ':':
			// delete tail string
			i--
			for ; i >= 0; i-- {
				if buf[i] == '"' {
					i--
					for ; i >= 0; i-- {
						if buf[i] == '"' {
							i--
							if i >= 0 && buf[i] == '\\' {
								i--
								continue
							}
							for ; i >= 0; i-- {
								// look for either a ',','{'
								switch buf[i] {
								case '{':
									return buf[:i+1], true
								case ',':
									return buf[:i], false
								}
							}
						}
					}
					break
				}
			}
			break loop
		}
	}
	return buf, false
}

var errNoChange = &errorType{"no change"}

func appendRawPaths(buf []byte, jstr string, paths []pathResult, raw string,
	stringify, del bool, cond Cond) ([]byte, error) {
	var err error
	var res gjson.Result
	var found bool
	if del {
		if paths[0].part == "-1" && !paths[0].force {
			res = gjson.Get(jstr, "#")
			if res.Int() > 0 {
				res = gjson.Get(jstr, strconv.FormatInt(int64(res.Int()-1), 10))
				found = true
			}
		}
	}
	if !found {
		res = gjson.Get(jstr, paths[0].gpart)
	}
	if res.Index > 0 {
		if len(paths) > 1 {
			buf = append(buf, jstr[:res.Index]...)
			buf, err = appendRawPaths(buf, res.Raw, paths[1:], raw,
				stringify, del, cond)
			if err != nil {
				return nil, err
			}
			buf = append(buf, jstr[res.Index+len(res.Raw):]...)
			return buf, nil
		}
		// The target exists at this real location. Verify the condition
		// against the value found here, in this same parse state, before
		// modifying any bytes.
		if !condSatisfied(cond, res, true) {
			return nil, errNoChange
		}
		buf = append(buf, jstr[:res.Index]...)
		var exidx int // additional forward stripping
		if del {
			var delNextComma bool
			buf, delNextComma = deleteTailItem(buf)
			if delNextComma {
				i, j := res.Index+len(res.Raw), 0
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
		} else {
			if stringify {
				buf = appendStringify(buf, raw)
			} else {
				buf = append(buf, raw...)
			}
		}
		buf = append(buf, jstr[res.Index+len(res.Raw)+exidx:]...)
		return buf, nil
	}
	if del {
		return nil, errNoChange
	}
	// The target does not exist; a new value would be created here. Only
	// conditions that accept a missing value may proceed.
	if !condSatisfied(cond, gjson.Result{}, false) {
		return nil, errNoChange
	}
	n, numeric := atoui(paths[0])
	isempty := true
	for i := 0; i < len(jstr); i++ {
		if jstr[i] > ' ' {
			isempty = false
			break
		}
	}
	if isempty {
		if numeric {
			jstr = "[]"
		} else {
			jstr = "{}"
		}
	}
	jsres := gjson.Parse(jstr)
	if jsres.Type != gjson.JSON {
		if numeric {
			jstr = "[]"
		} else {
			jstr = "{}"
		}
		jsres = gjson.Parse(jstr)
	}
	var comma bool
	for i := 1; i < len(jsres.Raw); i++ {
		if jsres.Raw[i] <= ' ' {
			continue
		}
		if jsres.Raw[i] == '}' || jsres.Raw[i] == ']' {
			break
		}
		comma = true
		break
	}
	switch jsres.Raw[0] {
	default:
		return nil, &errorType{"json must be an object or array"}
	case '{':
		end := len(jsres.Raw) - 1
		for ; end > 0; end-- {
			if jsres.Raw[end] == '}' {
				break
			}
		}
		buf = append(buf, jsres.Raw[:end]...)
		if comma {
			buf = append(buf, ',')
		}
		buf = appendBuild(buf, false, paths, raw, stringify)
		buf = append(buf, '}')
		return buf, nil
	case '[':
		var appendit bool
		if !numeric {
			if paths[0].part == "-1" && !paths[0].force {
				appendit = true
			} else {
				return nil, &errorType{
					"cannot set array element for non-numeric key '" +
						paths[0].part + "'"}
			}
		}
		if appendit {
			njson := trim(jsres.Raw)
			if njson[len(njson)-1] == ']' {
				njson = njson[:len(njson)-1]
			}
			buf = append(buf, njson...)
			if comma {
				buf = append(buf, ',')
			}

			buf = appendBuild(buf, true, paths, raw, stringify)
			buf = append(buf, ']')
			return buf, nil
		}
		buf = append(buf, '[')
		ress := jsres.Array()
		for i := 0; i < len(ress); i++ {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, ress[i].Raw...)
		}
		if len(ress) == 0 {
			buf = appendRepeat(buf, "null,", n-len(ress))
		} else {
			buf = appendRepeat(buf, ",null", n-len(ress))
			if comma {
				buf = append(buf, ',')
			}
		}
		buf = appendBuild(buf, true, paths, raw, stringify)
		buf = append(buf, ']')
		return buf, nil
	}
}

func isOptimisticPath(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] < '.' || path[i] > 'z' {
			return false
		}
		if path[i] > '9' && path[i] < 'A' {
			return false
		}
		if path[i] > 'z' {
			return false
		}
	}
	return true
}

// Set sets a json value for the specified path.
// A path is in dot syntax, such as "name.last" or "age".
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// An error is returned if the path is not valid.
//
// A path is a series of keys separated by a dot.
//
//  {
//    "name": {"first": "Tom", "last": "Anderson"},
//    "age":37,
//    "children": ["Sara","Alex","Jack"],
//    "friends": [
//      {"first": "James", "last": "Murphy"},
//      {"first": "Roger", "last": "Craig"}
//    ]
//  }
//  "name.last"          >> "Anderson"
//  "age"                >> 37
//  "children.1"         >> "Alex"
//
func Set(json, path string, value interface{}) (string, error) {
	return SetOptions(json, path, value, nil)
}

// SetBytes sets a json value for the specified path.
// If working with bytes, this method preferred over
// Set(string(data), path, value)
func SetBytes(json []byte, path string, value interface{}) ([]byte, error) {
	return SetBytesOptions(json, path, value, nil)
}

// SetRaw sets a raw json value for the specified path.
// This function works the same as Set except that the value is set as a
// raw block of json. This allows for setting premarshalled json objects.
func SetRaw(json, path, value string) (string, error) {
	return SetRawOptions(json, path, value, nil)
}

// SetRawOptions sets a raw json value for the specified path with options.
// This furnction works the same as SetOptions except that the value is set
// as a raw block of json. This allows for setting premarshalled json objects.
func SetRawOptions(json, path, value string, opts *Options) (string, error) {
	var optimistic bool
	if opts != nil {
		optimistic = opts.Optimistic
	}
	res, err := set(json, path, value, false, false, optimistic, false,
		CondAny)
	if err == errNoChange {
		return json, nil
	}
	return string(res), err
}

// SetRawBytes sets a raw json value for the specified path.
// If working with bytes, this method preferred over
// SetRaw(string(data), path, value)
func SetRawBytes(json []byte, path string, value []byte) ([]byte, error) {
	return SetRawBytesOptions(json, path, value, nil)
}

type dtype struct{}

// Delete deletes a value from json for the specified path.
func Delete(json, path string) (string, error) {
	return Set(json, path, dtype{})
}

// DeleteOptions deletes a value from json for the specified path, with the given options.
func DeleteOptions(json, path string, options *Options) (string, error) {
	return SetOptions(json, path, dtype{}, options)
}

// DeleteBytes deletes a value from json for the specified path.
func DeleteBytes(json []byte, path string) ([]byte, error) {
	return SetBytes(json, path, dtype{})
}

// DeleteBytesOptions deletes a value from json for the specified path, with the given options.
func DeleteBytesOptions(json []byte, path string, options *Options) ([]byte, error) {
	return SetBytesOptions(json, path, dtype{}, options)
}

type stringHeader struct {
	data unsafe.Pointer
	len  int
}

type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func set(jstr, path, raw string,
	stringify, del, optimistic, inplace bool, cond Cond) ([]byte, error) {
	if path == "" {
		return []byte(jstr), &errorType{"path cannot be empty"}
	}
	if !del && optimistic && isOptimisticPath(path) {
		res := gjson.Get(jstr, path)
		if res.Exists() && res.Index > 0 {
			sz := len(jstr) - len(res.Raw) + len(raw)
			if stringify {
				sz += 2
			}
			if inplace && sz <= len(jstr) {
				if !stringify || !mustMarshalString(raw) {
					jsonh := *(*stringHeader)(unsafe.Pointer(&jstr))
					jsonbh := sliceHeader{
						data: jsonh.data, len: jsonh.len, cap: jsonh.len}
					jbytes := *(*[]byte)(unsafe.Pointer(&jsonbh))
					if stringify {
						jbytes[res.Index] = '"'
						copy(jbytes[res.Index+1:], []byte(raw))
						jbytes[res.Index+1+len(raw)] = '"'
						copy(jbytes[res.Index+1+len(raw)+1:],
							jbytes[res.Index+len(res.Raw):])
					} else {
						copy(jbytes[res.Index:], []byte(raw))
						copy(jbytes[res.Index+len(raw):],
							jbytes[res.Index+len(res.Raw):])
					}
					return jbytes[:sz], nil
				}
				return []byte(jstr), nil
			}
			buf := make([]byte, 0, sz)
			buf = append(buf, jstr[:res.Index]...)
			if stringify {
				buf = appendStringify(buf, raw)
			} else {
				buf = append(buf, raw...)
			}
			buf = append(buf, jstr[res.Index+len(res.Raw):]...)
			return buf, nil
		}
	}
	var paths []pathResult
	r, simple := parsePath(path)
	if simple {
		paths = append(paths, r)
		for r.more {
			r, simple = parsePath(r.path)
			if !simple {
				break
			}
			paths = append(paths, r)
		}
	}
	if !simple {
		if del {
			return []byte(jstr),
				&errorType{"cannot delete value from a complex path"}
		}
		if cond != CondAny {
			return []byte(jstr),
				&errorType{"cannot use a condition with a complex path"}
		}
		return setComplexPath(jstr, path, raw, stringify)
	}
	njson, err := appendRawPaths(nil, jstr, paths, raw, stringify, del, cond)
	if err != nil {
		return []byte(jstr), err
	}
	return njson, nil
}

func setComplexPath(jstr, path, raw string, stringify bool) ([]byte, error) {
	res := gjson.Get(jstr, path)
	if !res.Exists() || !(res.Index != 0 || len(res.Indexes) != 0) {
		return []byte(jstr), errNoChange
	}
	if res.Index != 0 {
		njson := []byte(jstr[:res.Index])
		if stringify {
			njson = appendStringify(njson, raw)
		} else {
			njson = append(njson, raw...)
		}
		njson = append(njson, jstr[res.Index+len(res.Raw):]...)
		jstr = string(njson)
	}
	if len(res.Indexes) > 0 {
		type val struct {
			index int
			res   gjson.Result
		}
		vals := make([]val, 0, len(res.Indexes))
		res.ForEach(func(_, vres gjson.Result) bool {
			vals = append(vals, val{res: vres})
			return true
		})
		if len(res.Indexes) != len(vals) {
			return []byte(jstr), errNoChange
		}
		for i := 0; i < len(res.Indexes); i++ {
			vals[i].index = res.Indexes[i]
		}
		sort.SliceStable(vals, func(i, j int) bool {
			return vals[i].index > vals[j].index
		})
		for _, val := range vals {
			vres := val.res
			index := val.index
			njson := []byte(jstr[:index])
			if stringify {
				njson = appendStringify(njson, raw)
			} else {
				njson = append(njson, raw...)
			}
			njson = append(njson, jstr[index+len(vres.Raw):]...)
			jstr = string(njson)
		}
	}
	return []byte(jstr), nil
}

// SetOptions sets a json value for the specified path with options.
// A path is in dot syntax, such as "name.last" or "age".
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// An error is returned if the path is not valid.
func SetOptions(json, path string, value interface{},
	opts *Options) (string, error) {
	if opts != nil {
		if opts.ReplaceInPlace {
			// it's not safe to replace bytes in-place for strings
			// copy the Options and set options.ReplaceInPlace to false.
			nopts := *opts
			opts = &nopts
			opts.ReplaceInPlace = false
		}
	}
	jsonh := *(*stringHeader)(unsafe.Pointer(&json))
	jsonbh := sliceHeader{data: jsonh.data, len: jsonh.len, cap: jsonh.len}
	jsonb := *(*[]byte)(unsafe.Pointer(&jsonbh))
	res, err := SetBytesOptions(jsonb, path, value, opts)
	return string(res), err
}

// valueToRaw converts a user value into its raw json representation.
// It reports whether the value must be stringified (string/[]byte) and
// whether the value requests a delete (dtype). A marshaling error is
// returned before any json parsing or condition checking takes place.
func valueToRaw(value interface{}) (raw string, stringify, del bool,
	err error) {
	switch v := value.(type) {
	default:
		b, merr := jsongo.Marshal(value)
		if merr != nil {
			return "", false, false, merr
		}
		return *(*string)(unsafe.Pointer(&b)), false, false, nil
	case dtype:
		return "", false, true, nil
	case string:
		return v, true, false, nil
	case []byte:
		return *(*string)(unsafe.Pointer(&v)), true, false, nil
	case bool:
		if v {
			return "true", false, false, nil
		}
		return "false", false, false, nil
	case int8:
		return strconv.FormatInt(int64(v), 10), false, false, nil
	case int16:
		return strconv.FormatInt(int64(v), 10), false, false, nil
	case int32:
		return strconv.FormatInt(int64(v), 10), false, false, nil
	case int64:
		return strconv.FormatInt(int64(v), 10), false, false, nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), false, false, nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), false, false, nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), false, false, nil
	case uint64:
		return strconv.FormatUint(uint64(v), 10), false, false, nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64),
			false, false, nil
	case float64:
		return strconv.FormatFloat(float64(v), 'f', -1, 64),
			false, false, nil
	}
}

// SetBytesOptions sets a json value for the specified path with options.
// If working with bytes, this method preferred over
// SetOptions(string(data), path, value)
func SetBytesOptions(json []byte, path string, value interface{},
	opts *Options) ([]byte, error) {
	var optimistic, inplace bool
	if opts != nil {
		optimistic = opts.Optimistic
		inplace = opts.ReplaceInPlace
	}
	jstr := *(*string)(unsafe.Pointer(&json))
	raw, stringify, del, err := valueToRaw(value)
	if err != nil {
		return nil, err
	}
	res, err := set(jstr, path, raw, stringify, del, optimistic, inplace,
		CondAny)
	if err == errNoChange {
		return json, nil
	}
	return res, err
}

// SetRawBytesOptions sets a raw json value for the specified path with options.
// If working with bytes, this method preferred over
// SetRawOptions(string(data), path, value, opts)
func SetRawBytesOptions(json []byte, path string, value []byte,
	opts *Options) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	vstr := *(*string)(unsafe.Pointer(&value))
	var optimistic, inplace bool
	if opts != nil {
		optimistic = opts.Optimistic
		inplace = opts.ReplaceInPlace
	}
	res, err := set(jstr, path, vstr, false, false, optimistic, inplace,
		CondAny)
	if err == errNoChange {
		return json, nil
	}
	return res, err
}

// SetIf sets a json value for the specified path only when the condition
// cond holds for the existing value at that path. The condition is checked
// at the real target location during the same traversal that performs the
// update, so object keys (including numeric keys such as "0") and array
// indexes are distinguished by the actual container type. When the
// condition is not met the input json is returned unchanged and no bytes
// are modified. A value that cannot be marshaled to json is rejected with
// an error before the condition is evaluated.
func SetIf(json, path string, value interface{}, cond Cond) (string, error) {
	jsonh := *(*stringHeader)(unsafe.Pointer(&json))
	jsonbh := sliceHeader{data: jsonh.data, len: jsonh.len, cap: jsonh.len}
	jsonb := *(*[]byte)(unsafe.Pointer(&jsonbh))
	res, err := SetBytesIf(jsonb, path, value, cond)
	return string(res), err
}

// SetBytesIf sets a json value for the specified path only when the
// condition cond holds for the existing value at that path.
// If working with bytes, this method preferred over
// SetIf(string(data), path, value, cond)
func SetBytesIf(json []byte, path string, value interface{},
	cond Cond) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	raw, stringify, del, err := valueToRaw(value)
	if err != nil {
		return nil, err
	}
	res, err := set(jstr, path, raw, stringify, del, false, false, cond)
	if err == errNoChange {
		return json, nil
	}
	return res, err
}

// SetRawIf sets a raw json value for the specified path only when the
// condition cond holds for the existing value at that path. The raw value
// must be valid json; an invalid value is rejected with an error before
// the condition is evaluated and before any bytes are modified.
func SetRawIf(json, path, value string, cond Cond) (string, error) {
	if !jsongo.Valid([]byte(value)) {
		return json, &errorType{"invalid raw json value"}
	}
	res, err := set(json, path, value, false, false, false, false, cond)
	if err == errNoChange {
		return json, nil
	}
	return string(res), err
}

// SetRawBytesIf sets a raw json value for the specified path only when the
// condition cond holds for the existing value at that path. The raw value
// must be valid json; an invalid value is rejected with an error before
// the condition is evaluated and before any bytes are modified.
// If working with bytes, this method preferred over
// SetRawIf(string(data), path, string(value), cond)
func SetRawBytesIf(json []byte, path string, value []byte,
	cond Cond) ([]byte, error) {
	if !jsongo.Valid(value) {
		return json, &errorType{"invalid raw json value"}
	}
	jstr := *(*string)(unsafe.Pointer(&json))
	vstr := *(*string)(unsafe.Pointer(&value))
	res, err := set(jstr, path, vstr, false, false, false, false, cond)
	if err == errNoChange {
		return json, nil
	}
	return res, err
}

// DeleteIf deletes a value from json for the specified path only when the
// condition cond holds for the existing value at that path. The condition
// is checked at the real target location, honoring the actual type of each
// parent container along the path. When the condition is not met the input
// json is returned unchanged and no bytes are modified.
func DeleteIf(json, path string, cond Cond) (string, error) {
	return SetIf(json, path, dtype{}, cond)
}

// DeleteBytesIf deletes a value from json for the specified path only when
// the condition cond holds for the existing value at that path.
// If working with bytes, this method preferred over
// DeleteIf(string(data), path, cond)
func DeleteBytesIf(json []byte, path string, cond Cond) ([]byte, error) {
	return SetBytesIf(json, path, dtype{}, cond)
}
