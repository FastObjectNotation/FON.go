# FON — Fast Object Notation

[![CI](https://github.com/FastObjectNotation/FON.go/actions/workflows/ci.yml/badge.svg)](https://github.com/FastObjectNotation/FON.go/actions/workflows/ci.yml)

A fast, human-readable, line-oriented serialization format — a compact
alternative to JSON for record-style data; each line is one record; values are
typed and can nest.

## Features

- **Compact, readable wire format** — `key=type:value` pairs, one record per line.
- **Typed values** — integers, floats, bool, strings, raw bytes, nested objects,
  and arrays of any of them.
- **Nested objects and arrays of objects**, with a configurable maximum depth.
- **Parallel** dump serialization and deserialization, with a per-call thread limit.
- **Byte-oriented parsing** — reads straight from `[]byte` and tolerates a UTF-8
  byte order mark.
- **Editable in memory** — add, set, replace and remove fields and records, then save.
- **Z85 encoding** for raw blobs (5 ASCII characters per 4 bytes).
- **Explicit ownership** — `Collection` values are either owned or views, and a
  view whose data changed returns `ErrStaleView` instead of reading freed memory.

## Format

Each line is one record: a comma-separated list of `key=type:value` pairs. A
`.fon` file is a sequence of records, indexed by line number (0-based).

```
name=s:"John",age=i:30,balance=d:1234.56
scores=i:[95,87,92],tags=s:["admin","user"]
user=o:{id=i:42,name=s:"Bob",addr=o:{city=s:"NY",zip=i:10001}}
items=o:[{id=i:1,qty=i:5},{id=i:2,qty=i:3}]
blob=r:"Hello"
```

### Type codes

| Code | Go type      | Example                       |
|------|--------------|-------------------------------|
| `e`  | `uint8`      | `count=e:255`                 |
| `t`  | `int16`      | `year=t:2024`                 |
| `i`  | `int32`      | `id=i:42`                     |
| `u`  | `uint32`     | `flags=u:12345`               |
| `l`  | `int64`      | `ts=l:1700000000`             |
| `g`  | `uint64`     | `big=g:18446744073709551615`  |
| `f`  | `float32`    | `ratio=f:3.14`                |
| `d`  | `float64`    | `pi=d:3.141592653589793`      |
| `s`  | `string`     | `name=s:"Hello"`              |
| `b`  | `bool`       | `active=b:1`                  |
| `r`  | `[]byte` (Z85) | `data=r:"Hello"`            |
| `o`  | `*Collection` | `user=o:{id=i:1}`            |

Every primitive and string type also has an array form (`scores=i:[1,2,3]`),
and `o` supports both nested objects (`{...}`) and arrays of objects
(`[{...},{...}]`). Bool is written as `0` or `1`; floats may be `nan`, `inf`
and `-inf`.

Strings are double-quoted and may contain any character, including U+0000. The
escapes `\n \r \t \b \f \" \\` are used, and every other control character is
written as `\uXXXX` (for example `\u0000` or `\u001B`). Parsing decodes
`\uXXXX` as well. Keys are not escaped, so a key may not contain `=`, `"`, `{`,
`}`, `[`, `]`, CR or LF; see [Editing records](#editing-records).

## Install

```
go get github.com/FastObjectNotation/FON.go@latest
```

Building an application that imports this package needs a C compiler: `gcc` or
mingw-w64 on Windows, Xcode command line tools or `clang` on macOS, `gcc` on
Linux. Set `CGO_ENABLED=1`; it is the default when a C compiler is found.

**Native library.** Pre-built `fon_native` libraries are attached to each
[GitHub Release](https://github.com/FastObjectNotation/FON.go/releases). The
package does not embed linker flags, so point `CGO_LDFLAGS` at the directory
that holds the library before building an application that imports this package:

```bash
export CGO_ENABLED=1
export CGO_LDFLAGS="-L/path/to/lib -lfon_native"
```

At run time, put the directory on `PATH` on Windows, so `fon_native.dll` is
found. On Linux and macOS, add `-Wl,-rpath,/path/to/lib` to `CGO_LDFLAGS` or set
`LD_LIBRARY_PATH` (Linux) or `DYLD_LIBRARY_PATH` (macOS).

## Usage

### A single record

```go
package main

import (
	"fmt"

	fon "github.com/FastObjectNotation/FON.go"
)

func main() {
	c := fon.NewCollection()
	defer c.Close()

	c.AddInt("id", 42)
	c.AddString("name", "Test Item")
	c.AddDouble("price", 99.99)

	data, _ := c.SerializeToBytes()
	fmt.Println(string(data))
	// id=i:42,name=s:"Test Item",price=d:99.99

	parsed, _ := fon.DeserializeCollectionFromBytes(data)
	defer parsed.Close()

	id, _ := parsed.GetInt("id")
	name, _ := parsed.GetString("name")
	fmt.Printf("id=%d name=%s\n", id, name)
}
```

### Many records

```go
dump := fon.NewDump()
defer dump.Close()

for i := 0; i < 1000; i++ {
	row := fon.NewCollection()
	row.AddInt("id", int32(i))
	row.AddString("text", fmt.Sprintf("row %d", i))
	dump.Add(uint64(i), row)
	// dump now owns row; do not close it
}

serialized, _ := dump.SerializeToBytes(0) // 0 = use all cores

dump2, _ := fon.DeserializeDumpFromBytes(serialized, 0)
defer dump2.Close()

record, _ := dump2.Get(0)
// record is a view owned by dump2; do not close it
text, _ := record.GetString("text")
fmt.Println(text) // row 0
```

### Nested objects and arrays of objects

```go
address := fon.NewCollection()
address.AddString("city", "NY")
address.AddInt("zip", 10001)

user := fon.NewCollection()
defer user.Close()

user.AddInt("id", 42)
user.AddString("name", "Bob")
user.AddCollection("addr", address) // address is moved into user; do not close it

items := make([]*fon.Collection, 0, 2)
for i := int32(1); i <= 2; i++ {
	item := fon.NewCollection()
	item.AddInt("id", i)
	item.AddInt("qty", 5-i)
	items = append(items, item)
}
user.AddCollectionArray("items", items) // every item is moved into user

addr, _ := user.GetCollection("addr") // nested view; do not close it
city, _ := addr.GetString("city")
fmt.Println(city) // NY

views, _ := user.GetCollectionArray("items")
qty, _ := views[1].GetInt("qty")
fmt.Println(qty) // 3
```

### Editing records

Fields are added with `Add<Kind>`, which fails if the key exists, and written
with `Set<Kind>`, which inserts or replaces. Records of a dump follow the same
rules with `Add`, `Set` and `Replace` (which requires the id to exist).

```go
c := fon.NewCollection()
defer c.Close()

c.AddInt("count", 1)
c.SetInt("count", 2)
c.SetString("name", "Ada")

if err := c.AddInt("count", 3); errors.Is(err, fon.ErrAlreadyExists) {
	fmt.Println("count already exists")
}

removed, _ := c.Remove("name")
fmt.Println(removed) // true

d := fon.NewDump()
defer d.Close()

row := fon.NewCollection()
row.AddInt("v", 1)
d.Add(10, row) // d owns row now

replacement := fon.NewCollection()
replacement.AddInt("v", 2)
d.Replace(10, replacement) // fails with ErrNotFound if id 10 is absent

rec, _ := d.Get(10)
rec.SetInt("v", 3) // a record view stays valid after its own edits

gone, _ := d.Remove(10)
fmt.Println(gone) // true
count, _ := d.RemoveMany([]uint64{10, 11})
fmt.Println(count) // 0: id 10 is already gone and id 11 was never added
```

Ids are stable while the dump is in memory. Serializing writes one line per
record in ascending id order, and deserializing numbers the lines from 0. A dump
with ids 5 and 9 is saved as two lines and loads back with ids 0 and 1.

A key that contains `=`, `"`, `{`, `}`, `[`, `]`, CR or LF is rejected with a
`*FonError` whose `Code` is `ResultInvalidArgument`. A value passed with such a
key stays owned by the caller.

### Binary data and parse options

```go
c := fon.NewCollection()
defer c.Close()

c.AddRaw("blob", []byte{0x86, 0x4f, 0xd2, 0x6f})

serialized, _ := c.SerializeToString()
fmt.Println(serialized) // blob=r:"Hello"

data, _ := c.GetRaw("blob")
fmt.Println(data) // [134 79 210 111]

fon.SetMaxDepth(128)   // nesting limit used when parsing; default 64
fon.SetRawUnpack(true) // decode raw blobs while parsing; default off
```

Raw values are written as Z85 text, five ASCII characters per four bytes, and
`GetRaw` always returns the decoded bytes. Both parse settings apply to the
whole process. Dump serialization and deserialization take a `maxThreads`
argument: 0 or less uses all cores, and a positive value caps the threads used
by that call.

## Memory and lifetime

**Owned values.** `NewCollection`, `Clone` and `DeserializeCollectionFromBytes`
return owned collections. `NewDump`, `DeserializeDumpFromBytes` and
`DeserializeDumpFromFile` return owned dumps. Owned values hold native memory,
so call `Close` when you are done with them. The garbage collector releases an
unreachable owned value as well, but when that happens is not defined.

**Views.** `Dump.Get` and `Dump.Range` return record views. `GetCollection` and
`GetCollectionArray` return nested views. A view never owns memory: do not close
it, and do not insert it into another container. Call `Clone` on a view to get
an owned copy. A view keeps its owner (the dump, or the owned root collection)
reachable for as long as the view is in use.

**Moving values into containers.** Passing an owned collection to
`AddCollection`, `SetCollection`, `AddCollectionArray`, `SetCollectionArray`,
`Dump.Add`, `Dump.Set` or `Dump.Replace` moves it into the container. On success
the moved value is unusable and reports `ErrClosed`, so do not close it. On
error the caller keeps ownership and the value stays usable. A view cannot be
inserted (call `Clone` first), a collection cannot be inserted into itself or
into its own nested view, and the same collection cannot appear twice in one
array.

**Stale views.** A record view stays valid through edits to its own record and
through edits to other records. After its record is removed or replaced, it
returns an error matching `ErrStaleView`. A nested view becomes stale when any
field of its record changes, or, for a nested view of an owned root collection,
when that root is changed. Editing through a nested view keeps that view valid
and makes the other nested views of the same parent stale. When an access reports `ErrStaleView`, get a
new view from the parent.

**Closing.** Closing a dump, or an owned root collection, makes its views report
`ErrClosed`. `Close` does nothing on a view or on a value that was moved into a
container.

**Errors.** Failures are reported as `error` values that work with `errors.Is`:
an absent key or id matches `ErrNotFound`, a wrong type matches
`ErrTypeMismatch`, adding an existing key or id matches `ErrAlreadyExists`, a
stale view matches `ErrStaleView`, and use after close or move matches
`ErrClosed`. Other failures (parse, write, file, invalid argument) are
`*FonError` values with a `Code` and a `Message`.

## API

### Module functions

| Function | Description |
|----------|-------------|
| `NativeVersion() string` | Version of the native library, for example `0.4.0`. |
| `SetRawUnpack(enable bool)` | Decode raw blobs while parsing. Off by default. Applies to the whole process. |
| `SetMaxDepth(depth int)` | Nesting limit used when parsing. Default 64; values below 1 are clamped to 1. |
| `DeserializeCollectionFromBytes(data []byte) (*Collection, error)` | Parse one record into a new owned collection. |
| `DeserializeDumpFromBytes(data []byte, maxThreads int) (*Dump, error)` | Parse a multi-line buffer into a new owned dump. |
| `DeserializeDumpFromFile(path string, maxThreads int) (*Dump, error)` | Read a `.fon` file into a new owned dump. A missing file reports `ResultFileNotFound`. |

### Collection

| Member | Description |
|--------|-------------|
| `NewCollection() *Collection` | New empty owned collection. |
| `Close()` | Release an owned collection. |
| `Clone() (*Collection, error)` | Owned deep copy of an owned collection or a view. |
| `Size() int64` | Number of keys. |
| `Keys() ([]string, error)` | Keys in insertion order. |
| `Kind(key string) (Kind, bool)` | Type and array flag of the value under `key`; the boolean is false if the key is absent. |
| `Has(key string) bool` | Whether `key` is present. |
| `Remove(key string) (bool, error)` | Delete `key`; reports whether it was present. |
| `Get(key string) (any, error)` | Read a value whose dynamic type matches the typed getter for its kind. |
| `SerializeToBytes() ([]byte, error)` | Serialize as one record without a trailing newline. |
| `SerializeToString() (string, error)` | Same as `SerializeToBytes`, returned as a string. |
| `AddCollection(key string, child *Collection) error` | Move `child` under `key`. |
| `SetCollection(key string, child *Collection) error` | Move `child` under `key`, replacing any value there. |
| `GetCollection(key string) (*Collection, error)` | Nested view of the collection under `key`. |
| `AddCollectionArray(key string, children []*Collection) error` | Move every child under `key` as an array. |
| `SetCollectionArray(key string, children []*Collection) error` | Same as `AddCollectionArray`, replacing any value there. |
| `GetCollectionArray(key string) ([]*Collection, error)` | Nested views of every element of the collection array under `key`. |

Typed values use `Add<Kind>` (fails if the key exists), `Set<Kind>` (inserts or
replaces) and `Get<Kind>` (fails with `ErrNotFound` or `ErrTypeMismatch`):

| Kind | Go type | Methods |
|------|---------|---------|
| Byte | `uint8` | `AddByte`, `SetByte`, `GetByte` |
| Short | `int16` | `AddShort`, `SetShort`, `GetShort` |
| Int | `int32` | `AddInt`, `SetInt`, `GetInt` |
| UInt | `uint32` | `AddUInt`, `SetUInt`, `GetUInt` |
| Long | `int64` | `AddLong`, `SetLong`, `GetLong` |
| ULong | `uint64` | `AddULong`, `SetULong`, `GetULong` |
| Float | `float32` | `AddFloat`, `SetFloat`, `GetFloat` |
| Double | `float64` | `AddDouble`, `SetDouble`, `GetDouble` |
| Bool | `bool` | `AddBool`, `SetBool`, `GetBool` |
| String | `string` | `AddString`, `SetString`, `GetString` |
| Raw | `[]byte` | `AddRaw`, `SetRaw`, `GetRaw` |
| ByteArray | `[]uint8` | `AddByteArray`, `SetByteArray`, `GetByteArray` |
| ShortArray | `[]int16` | `AddShortArray`, `SetShortArray`, `GetShortArray` |
| IntArray | `[]int32` | `AddIntArray`, `SetIntArray`, `GetIntArray` |
| UIntArray | `[]uint32` | `AddUIntArray`, `SetUIntArray`, `GetUIntArray` |
| LongArray | `[]int64` | `AddLongArray`, `SetLongArray`, `GetLongArray` |
| ULongArray | `[]uint64` | `AddULongArray`, `SetULongArray`, `GetULongArray` |
| FloatArray | `[]float32` | `AddFloatArray`, `SetFloatArray`, `GetFloatArray` |
| DoubleArray | `[]float64` | `AddDoubleArray`, `SetDoubleArray`, `GetDoubleArray` |
| BoolArray | `[]bool` | `AddBoolArray`, `SetBoolArray`, `GetBoolArray` |
| StringArray | `[]string` | `AddStringArray`, `SetStringArray`, `GetStringArray` |

`Kind` is a struct with the fields `Type` (`TypeByte`, `TypeShort`, `TypeInt`,
`TypeUInt`, `TypeLong`, `TypeULong`, `TypeFloat`, `TypeDouble`, `TypeBool`,
`TypeString`, `TypeRaw` or `TypeObject`) and `Array bool`.

### Dump

| Member | Description |
|--------|-------------|
| `NewDump() *Dump` | New empty owned dump. |
| `Close()` | Release the dump. |
| `Size() int64` | Number of records. |
| `Add(id uint64, collection *Collection) error` | Move `collection` in under `id`. Fails with `ErrAlreadyExists` if the id is present. |
| `Set(id uint64, collection *Collection) error` | Move `collection` in under `id`, replacing any record with that id. |
| `Replace(id uint64, collection *Collection) error` | Move `collection` in in place of an existing record. Fails with `ErrNotFound` if the id is absent. |
| `Remove(id uint64) (bool, error)` | Delete the record with `id`; reports whether it was present. |
| `RemoveMany(ids []uint64) (int, error)` | Delete every listed id that is present; returns how many were removed. Absent and duplicate ids are ignored. |
| `Has(id uint64) bool` | Whether a record with `id` is present. |
| `Ids() ([]uint64, error)` | Ids of all records in ascending order. |
| `Get(id uint64) (*Collection, error)` | Record view of the record with `id`. Fails with `ErrNotFound` if the id is absent. |
| `Range(fn func(id uint64, c *Collection) bool) error` | Call `fn` with a record view of each record in ascending id order, until `fn` returns false. |
| `SerializeToBytes(maxThreads int) ([]byte, error)` | Serialize one record per line. |
| `SerializeToFile(path string, maxThreads int) error` | Write the dump to a `.fon` file. |

## Build and test

Requirements: Go 1.22 or later, `cargo` with a stable toolchain, and a C compiler.

Build the native library first:

```bash
cargo build --release --manifest-path native/Cargo.toml
```

On Linux and macOS, from the repository root:

```bash
CGO_ENABLED=1 \
CGO_LDFLAGS="-L$PWD/native/target/release -lfon_native -Wl,-rpath,$PWD/native/target/release" \
go test -count=1 ./...
```

On Windows, from PowerShell in the repository root:

```powershell
$env:CGO_ENABLED = "1"
$env:CGO_LDFLAGS = "-L$PWD\native\target\release -lfon_native"
$env:PATH = "$PWD\native\target\release;" + $env:PATH
go test -count=1 ./...
```
