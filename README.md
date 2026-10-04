<p align="center">
<img 
    src="logo.png" 
    width="240" height="78" border="0" alt="SJSON">
<br>
<a href="https://godoc.org/github.com/tidwall/sjson"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>
</p>

<p align="center">set a json value quickly</p>

SJSON is a Go package that provides a [very fast](#performance) and simple way to set a value in a json document.
For quickly retrieving json values check out [GJSON](https://github.com/tidwall/gjson).

For a command line interface check out [JJ](https://github.com/tidwall/jj).

Getting Started
===============

Installing
----------

To start using SJSON, install Go and run `go get`:

```sh
$ go get -u github.com/tidwall/sjson
```

This will retrieve the library.

Set a value
-----------
Set sets the value for the specified path. 
A path is in dot syntax, such as "name.last" or "age". 
This function expects that the json is well-formed and validated. 
Invalid json will not panic, but it may return back unexpected results.
Invalid paths may return an error.

```go
package main

import "github.com/tidwall/sjson"

const json = `{"name":{"first":"Janet","last":"Prichard"},"age":47}`

func main() {
	value, _ := sjson.Set(json, "name.last", "Anderson")
	println(value)
}
```

This will print:

```json
{"name":{"first":"Janet","last":"Anderson"},"age":47}
```

Path syntax
-----------

A path is a series of keys separated by a dot.
The dot and colon characters can be escaped with ``\``.

```json
{
  "name": {"first": "Tom", "last": "Anderson"},
  "age":37,
  "children": ["Sara","Alex","Jack"],
  "fav.movie": "Deer Hunter",
  "friends": [
	{"first": "James", "last": "Murphy"},
	{"first": "Roger", "last": "Craig"}
  ]
}
```
```
"name.last"          >> "Anderson"
"age"                >> 37
"children.1"         >> "Alex"
"friends.1.last"     >> "Craig"
```

The `-1` key can be used to append a value to an existing array:

```
"children.-1"  >> appends a new value to the end of the children array
```

Normally number keys are used to modify arrays, but it's possible to force a numeric object key by using the colon character:

```json
{
  "users":{
    "2313":{"name":"Sara"},
    "7839":{"name":"Andy"}
  }
}
```

A colon path would look like:

```
"users.:2313.name"    >> "Sara"
```

Supported types
---------------

Pretty much any type is supported:

```go
sjson.Set(`{"key":true}`, "key", nil)
sjson.Set(`{"key":true}`, "key", false)
sjson.Set(`{"key":true}`, "key", 1)
sjson.Set(`{"key":true}`, "key", 10.5)
sjson.Set(`{"key":true}`, "key", "hello")
sjson.Set(`{"key":true}`, "key", []string{"hello", "world"})
sjson.Set(`{"key":true}`, "key", map[string]interface{}{"hello":"world"})
```

When a type is not recognized, SJSON will fallback to the `encoding/json` Marshaller.


Examples
--------

Set a value from empty document:
```go
value, _ := sjson.Set("", "name", "Tom")
println(value)

// Output:
// {"name":"Tom"}
```

Set a nested value from empty document:
```go
value, _ := sjson.Set("", "name.last", "Anderson")
println(value)

// Output:
// {"name":{"last":"Anderson"}}
```

Set a new value:
```go
value, _ := sjson.Set(`{"name":{"last":"Anderson"}}`, "name.first", "Sara")
println(value)

// Output:
// {"name":{"first":"Sara","last":"Anderson"}}
```

Update an existing value:
```go
value, _ := sjson.Set(`{"name":{"last":"Anderson"}}`, "name.last", "Smith")
println(value)

// Output:
// {"name":{"last":"Smith"}}
```

Set a new array value:
```go
value, _ := sjson.Set(`{"friends":["Andy","Carol"]}`, "friends.2", "Sara")
println(value)

// Output:
// {"friends":["Andy","Carol","Sara"]
```

Append an array value by using the `-1` key in a path:
```go
value, _ := sjson.Set(`{"friends":["Andy","Carol"]}`, "friends.-1", "Sara")
println(value)

// Output:
// {"friends":["Andy","Carol","Sara"]
```

Append an array value that is past the end:
```go
value, _ := sjson.Set(`{"friends":["Andy","Carol"]}`, "friends.4", "Sara")
println(value)

// Output:
// {"friends":["Andy","Carol",null,null,"Sara"]
```

Delete a value:
```go
value, _ := sjson.Delete(`{"name":{"first":"Sara","last":"Anderson"}}`, "name.first")
println(value)

// Output:
// {"name":{"last":"Anderson"}}
```

Delete an array value:
```go
value, _ := sjson.Delete(`{"friends":["Andy","Carol"]}`, "friends.1")
println(value)

// Output:
// {"friends":["Andy"]}
```

Delete the last array value:
```go
value, _ := sjson.Delete(`{"friends":["Andy","Carol"]}`, "friends.-1")
println(value)

// Output:
// {"friends":["Andy"]}
```

Batch editing
-------------

`SetBytesBatch` (and its string variant `SetBatch`) applies an ordered
list of set/delete operations in one call:

```go
res, err := sjson.SetBytesBatch(
	[]byte(`{"arr":["a","b","c"]}`),
	[]sjson.BatchOp{
		sjson.BatchDelete("arr.0"),   // delete the first element
		sjson.BatchSet("arr.1", "z"), // modifies the original "c"
	},
)
// res.JSON == {"arr":["b","z"]}
```

Each operation runs against the document produced by the previous
operation, reusing the same raw byte splicing as `SetBytes` and
`DeleteBytes`. If an earlier operation changed the document length or
shifted array indices, the next operation is relocated against the new
structure; stale offsets are never reused.

The batch is atomic: if any operation has an invalid path or a value
that is not valid JSON, a `*BatchError` (with the failing operation
index) is returned and `res.JSON` holds the complete original input —
no partial result is published. The input bytes are never modified.

An empty path targets the root document, so `BatchSetRaw("", "[1,2,3]")`
replaces the whole document (deleting the root is an error).

For every executed operation, `res.Ops[i]` reports the minimal changed
region as half-open byte intervals:

- `Before` — the changed bytes in the document before the operation
  (the old value for a set, the removed bytes for a delete, a
  zero-width insertion point for a new member).
- `After` — the changed bytes in the document after the operation
  (the new value for a set, a zero-width collapse point for a delete).

All intervals are relative to the document version they refer to, so
`Ops[i].After` and `Ops[i+1].Before` index into the same intermediate
document.

Run only the batch tests with:

```sh
go test -run Batch ./
```

## Performance

Benchmarks of SJSON alongside [encoding/json](https://golang.org/pkg/encoding/json/), 
[ffjson](https://github.com/pquerna/ffjson), 
[EasyJSON](https://github.com/mailru/easyjson),
and [Gabs](https://github.com/Jeffail/gabs)

```
Benchmark_SJSON-8                  	 3000000	       805 ns/op	    1077 B/op	       3 allocs/op
Benchmark_SJSON_ReplaceInPlace-8   	 3000000	       449 ns/op	       0 B/op	       0 allocs/op
Benchmark_JSON_Map-8               	  300000	     21236 ns/op	    6392 B/op	     150 allocs/op
Benchmark_JSON_Struct-8            	  300000	     14691 ns/op	    1789 B/op	      24 allocs/op
Benchmark_Gabs-8                   	  300000	     21311 ns/op	    6752 B/op	     150 allocs/op
Benchmark_FFJSON-8                 	  300000	     17673 ns/op	    3589 B/op	      47 allocs/op
Benchmark_EasyJSON-8               	 1500000	      3119 ns/op	    1061 B/op	      13 allocs/op
```

JSON document used:

```json
{
  "widget": {
    "debug": "on",
    "window": {
      "title": "Sample Konfabulator Widget",
      "name": "main_window",
      "width": 500,
      "height": 500
    },
    "image": { 
      "src": "Images/Sun.png",
      "hOffset": 250,
      "vOffset": 250,
      "alignment": "center"
    },
    "text": {
      "data": "Click Here",
      "size": 36,
      "style": "bold",
      "vOffset": 100,
      "alignment": "center",
      "onMouseUp": "sun1.opacity = (sun1.opacity / 100) * 90;"
    }
  }
}    
```

Each operation was rotated though one of the following search paths:

```
widget.window.name
widget.image.hOffset
widget.text.onMouseUp
```

*These benchmarks were run on a MacBook Pro 15" 2.8 GHz Intel Core i7 using Go 1.7 and can be be found [here](https://github.com/tidwall/sjson-benchmarks)*.

## Contact
Josh Baker [@tidwall](http://twitter.com/tidwall)

## License

SJSON source code is available under the MIT [License](/LICENSE).
