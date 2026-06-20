# FON — Fast Object Notation (Go binding)

[![CI](https://github.com/FastObjectNotation/FON.go/actions/workflows/ci.yml/badge.svg)](https://github.com/FastObjectNotation/FON.go/actions/workflows/ci.yml)

Go bindings for **FON** (Fast Object Notation) — a fast, human-readable,
line-oriented serialization format. Each line is one record; values are typed
and can nest.

The binding wraps the `fon_native` Rust cdylib via **cgo**. The Rust library
is compiled from source in the `native/` directory.

## Features

- **Compact, readable wire format** — `key=type:value` pairs, one record per line.
- **Typed values** — numeric/bool/string primitives, binary blobs, nested
  objects, and arrays of any of them.
- **Nested objects & arrays of objects**, with a configurable maximum depth.
- **Parallel** dump serialization and deserialization via [Rayon](https://crates.io/crates/rayon).
- **Byte-oriented parsing** — reads straight from `[]byte`, BOM tolerant.
- **Z85 binary encoding** for raw blobs (5 ASCII chars per 4 bytes).
- **Idiomatic Go API** — `Collection` and `Dump` types with clear ownership
  rules, `runtime.SetFinalizer`-backed resource management.

## Format

Each line is one record: a comma-separated list of `key=type:value` pairs. A
`.fon` file is a sequence of records, indexed by line number (0-based).

```
name=s:"John",age=i:30,balance=d:1234.56
scores=i:[95,87,92],tags=s:["admin","user"]
user=o:{id=i:42,name=s:"Bob",addr=o:{city=s:"NY",zip=i:10001}}
items=o:[{id=i:1,qty=i:5},{id=i:2,qty=i:3}]
blob=r:"nm=QNzv..."
```

### Type codes

| Code | Type            | Example                       |
|------|-----------------|-------------------------------|
| `e`  | `uint8`         | `count=e:255`                 |
| `t`  | `int16`         | `year=t:2024`                 |
| `i`  | `int32`         | `id=i:42`                     |
| `u`  | `uint32`        | `flags=u:12345`               |
| `l`  | `int64`         | `ts=l:1700000000`             |
| `g`  | `uint64`        | `big=g:18446744073709551615`  |
| `f`  | `float32`       | `ratio=f:3.14`                |
| `d`  | `float64`       | `pi=d:3.141592653589793`      |
| `s`  | `string`        | `name=s:"Hello"`              |
| `b`  | `bool`          | `active=b:1`                  |
| `r`  | raw (Z85)       | `data=r:"nm=QNzv"`            |
| `o`  | nested object   | `user=o:{id=i:1}`             |

Every primitive and string type also has an array form (`scores=i:[1,2,3]`),
and `o` supports both nested objects (`{...}`) and arrays of objects
(`[{...},{...}]`). Strings are double-quoted with `\n \r \t \b \f \" \\` escapes.

## Install

```
go get github.com/FastObjectNotation/FON.go
```

> **cgo prerequisite:** a C compiler must be present (`gcc` / `mingw-w64` on
> Windows, Xcode CLT / `clang` on macOS, `gcc` on Linux). CGO\_ENABLED=1 is
> required (the Go default when a C compiler is available).

> **Native library:** build the Rust cdylib first and ensure
> `native/target/release/fon_native.dll` (Windows) /
> `libfon_native.so` (Linux) / `libfon_native.dylib` (macOS) is on the
> runtime library search path. Pre-built binaries are attached to each
> [GitHub Release](https://github.com/FastObjectNotation/FON.go/releases).

### Build the native library from source

```bash
cargo build --release --manifest-path native/Cargo.toml
```

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

### Many records (Dump)

```go
dump := fon.NewDump()
defer dump.Close()

for i := 0; i < 1000; i++ {
    row := fon.NewCollection()
    row.AddInt("id", int32(i))
    row.AddString("text", fmt.Sprintf("row %d", i))
    dump.Add(uint64(i), row)
    // row is now owned by dump — do not call row.Close()
}

serialized, _ := dump.SerializeToBytes(0) // 0 = use global Rayon pool

dump2, _ := fon.DeserializeDumpFromBytes(serialized, 0)
defer dump2.Close()

col, _ := dump2.Get(0)
// col is borrowed from dump2 — do NOT close it
text, _ := col.GetString("text")
fmt.Println(text) // row 0
```

### Version check

```go
fmt.Println(fon.NativeVersion()) // 0.2.1
```

## Build

```bash
# 1. Build the Rust cdylib:
cargo build --release --manifest-path native/Cargo.toml

# 2. Run Go tests (requires gcc / mingw):
go test -v ./...
```

## Ownership rules

| Call | Ownership outcome |
|------|-------------------|
| `NewCollection()` | Caller owns; must `Close()` |
| `NewDump()` | Caller owns; must `Close()` |
| `Dump.Add(id, col)` | `col` transferred to dump; do NOT close `col` |
| `Collection.AddCollection(key, child)` | `child` transferred to parent; do NOT close `child` |
| `Dump.Get(index)` | Borrowed; do NOT close |
| `Collection.GetCollection(key)` | Borrowed; do NOT close |
| `DeserializeDumpFromBytes(...)` | Caller owns; must `Close()` |
| `DeserializeCollectionFromBytes(...)` | Caller owns; must `Close()` |
