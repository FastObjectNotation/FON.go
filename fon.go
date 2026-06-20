// Package fon provides Go bindings for the FON (Fast Object Notation)
// serialization library via cgo, wrapping the fon_native Rust cdylib.
//
// Build prerequisites:
//   - A C compiler (gcc / mingw-w64 on Windows, clang on macOS/Linux)
//   - The fon_native shared library built beforehand:
//     cargo build --release --manifest-path native/Cargo.toml
//
// The shared library is loaded from native/target/release/ relative to the
// source tree at link time (via the #cgo LDFLAGS directive below).
// Consumers of the compiled package must ensure fon_native.dll / libfon_native.so
// is on the runtime library search path.
package fon

/*
// fon.h is copied to the package root so cgo can find it without a path
// that contains the '#' character (which cgo cannot handle in CFLAGS -I paths).
// The LDFLAGS are intentionally omitted here; callers must supply them via the
// CGO_LDFLAGS environment variable or a wrapper build script:
//
//   Windows: set CGO_LDFLAGS=-L<abs-path-to-native/target/release> -lfon_native
//   Linux:   export CGO_LDFLAGS="-L<abs-path-to-native/target/release> -lfon_native -ldl -lm"
//   macOS:   export CGO_LDFLAGS="-L<abs-path-to-native/target/release> -lfon_native"
//
// In CI the workflows set CGO_LDFLAGS automatically.
#include "fon.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)


// ResultCode mirrors the FON_* result constants from fon.h.
const (
	ResultOK              = C.FON_OK
	ResultFileNotFound    = C.FON_ERROR_FILE_NOT_FOUND
	ResultParseFailed     = C.FON_ERROR_PARSE_FAILED
	ResultWriteFailed     = C.FON_ERROR_WRITE_FAILED
	ResultInvalidArgument = C.FON_ERROR_INVALID_ARGUMENT
)


// fonError converts a C FonError into a Go error.
func fonError(e C.FonError) error {
	msg := C.GoString(&e.message[0])
	if msg == "" {
		msg = fmt.Sprintf("fon error code %d", int(e.code))
	}
	return errors.New(msg)
}


// checkRC checks a return code and the associated FonError; returns nil on success.
func checkRC(rc C.int32_t, e C.FonError) error {
	if rc == C.FON_OK {
		return nil
	}
	return fonError(e)
}


// NativeVersion returns the version string reported by the native library (e.g. "0.2.1").
func NativeVersion() string {
	return C.GoString(C.fon_version())
}


// SetRawUnpack enables (true) or disables (false) raw-blob unpacking during
// deserialization. The setting is global within the native library.
func SetRawUnpack(enable bool) {
	v := C.int32_t(0)
	if enable {
		v = 1
	}
	C.fon_set_raw_unpack(v)
}


// SetMaxDepth sets the maximum nesting depth for deserialization (clamped to >= 1).
// The setting is global within the native library.
func SetMaxDepth(depth int) {
	C.fon_set_max_depth(C.int32_t(depth))
}


// =============================================================================
// Collection
// =============================================================================

// Collection is an opaque handle to a FON record (key → value map).
//
// Ownership model:
//   - A Collection returned by NewCollection or DeserializeCollectionFromBytes
//     is owned by the caller and must be freed via Close when no longer needed.
//   - A Collection returned by Dump.Get is a BORROWED handle — do not Close it.
//   - After a successful Dump.Add or Collection.AddCollection call, ownership
//     transfers to the parent; do not Close the child afterwards.
type Collection struct {
	ptr    unsafe.Pointer
	owned  bool
}


// NewCollection allocates a new, empty Collection. The caller is responsible
// for calling Close when done (unless ownership is transferred to a Dump or
// parent Collection).
func NewCollection() *Collection {
	p := C.fon_collection_create()
	c := &Collection{ptr: p, owned: true}
	runtime.SetFinalizer(c, (*Collection).finalizer)
	return c
}


// Close frees the underlying native Collection. Only call this if the Collection
// is owned by the caller (i.e., not obtained via Dump.Get or transferred via
// Dump.Add / Collection.AddCollection).
func (c *Collection) Close() {
	if c.ptr != nil && c.owned {
		C.fon_collection_free(c.ptr)
		c.ptr = nil
	}
	runtime.SetFinalizer(c, nil)
}


func (c *Collection) finalizer() {
	c.Close()
}


// Size returns the number of key-value pairs in the collection.
func (c *Collection) Size() int64 {
	return int64(C.fon_collection_size(c.ptr))
}


// AddInt stores a 32-bit integer under key.
func (c *Collection) AddInt(key string, value int32) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	rc := C.fon_collection_add_int(c.ptr, cKey, C.int32_t(value), &e)
	return checkRC(rc, e)
}


// AddLong stores a 64-bit integer under key.
func (c *Collection) AddLong(key string, value int64) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	rc := C.fon_collection_add_long(c.ptr, cKey, C.int64_t(value), &e)
	return checkRC(rc, e)
}


// AddFloat stores a 32-bit float under key.
func (c *Collection) AddFloat(key string, value float32) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	rc := C.fon_collection_add_float(c.ptr, cKey, C.float(value), &e)
	return checkRC(rc, e)
}


// AddDouble stores a 64-bit float under key.
func (c *Collection) AddDouble(key string, value float64) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	rc := C.fon_collection_add_double(c.ptr, cKey, C.double(value), &e)
	return checkRC(rc, e)
}


// AddBool stores a boolean under key.
func (c *Collection) AddBool(key string, value bool) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	v := C.int32_t(0)
	if value {
		v = 1
	}
	var e C.FonError
	rc := C.fon_collection_add_bool(c.ptr, cKey, v, &e)
	return checkRC(rc, e)
}


// AddString stores a UTF-8 string under key.
func (c *Collection) AddString(key string, value string) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	cVal := C.CString(value)
	defer C.free(unsafe.Pointer(cVal))
	var e C.FonError
	rc := C.fon_collection_add_string(c.ptr, cKey, cVal, &e)
	return checkRC(rc, e)
}


// AddIntArray stores a slice of int32 under key.
func (c *Collection) AddIntArray(key string, values []int32) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var ptr *C.int32_t
	if len(values) > 0 {
		ptr = (*C.int32_t)(unsafe.Pointer(&values[0]))
	}
	var e C.FonError
	rc := C.fon_collection_add_int_array(c.ptr, cKey, ptr, C.int64_t(len(values)), &e)
	return checkRC(rc, e)
}


// AddFloatArray stores a slice of float32 under key.
func (c *Collection) AddFloatArray(key string, values []float32) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var ptr *C.float
	if len(values) > 0 {
		ptr = (*C.float)(unsafe.Pointer(&values[0]))
	}
	var e C.FonError
	rc := C.fon_collection_add_float_array(c.ptr, cKey, ptr, C.int64_t(len(values)), &e)
	return checkRC(rc, e)
}


// AddCollection nests child under key inside c.
//
// OWNERSHIP: on success, child is owned by c. The caller MUST NOT call
// child.Close() and MUST NOT pass child to any other function.
func (c *Collection) AddCollection(key string, child *Collection) error {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	rc := C.fon_collection_add_collection(c.ptr, cKey, child.ptr, &e)
	if rc == C.FON_OK {
		// Ownership transferred: prevent the finalizer from double-freeing.
		child.owned = false
		runtime.SetFinalizer(child, nil)
	}
	return checkRC(rc, e)
}


// GetInt reads a 32-bit integer stored under key.
func (c *Collection) GetInt(key string) (int32, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var v C.int32_t
	var e C.FonError
	rc := C.fon_collection_get_int(c.ptr, cKey, &v, &e)
	if err := checkRC(rc, e); err != nil {
		return 0, err
	}
	return int32(v), nil
}


// GetLong reads a 64-bit integer stored under key.
func (c *Collection) GetLong(key string) (int64, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var v C.int64_t
	var e C.FonError
	rc := C.fon_collection_get_long(c.ptr, cKey, &v, &e)
	if err := checkRC(rc, e); err != nil {
		return 0, err
	}
	return int64(v), nil
}


// GetFloat reads a 32-bit float stored under key.
func (c *Collection) GetFloat(key string) (float32, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var v C.float
	var e C.FonError
	rc := C.fon_collection_get_float(c.ptr, cKey, &v, &e)
	if err := checkRC(rc, e); err != nil {
		return 0, err
	}
	return float32(v), nil
}


// GetDouble reads a 64-bit float stored under key.
func (c *Collection) GetDouble(key string) (float64, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var v C.double
	var e C.FonError
	rc := C.fon_collection_get_double(c.ptr, cKey, &v, &e)
	if err := checkRC(rc, e); err != nil {
		return 0, err
	}
	return float64(v), nil
}


// GetBool reads a boolean stored under key.
func (c *Collection) GetBool(key string) (bool, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var v C.int32_t
	var e C.FonError
	rc := C.fon_collection_get_bool(c.ptr, cKey, &v, &e)
	if err := checkRC(rc, e); err != nil {
		return false, err
	}
	return v != 0, nil
}


// GetString reads a UTF-8 string stored under key.
// It uses a 4 KiB stack buffer; larger values are handled by a heap allocation.
func (c *Collection) GetString(key string) (string, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError

	// Initial attempt with a 4096-byte buffer.
	const initialSize = 4096
	buf := make([]byte, initialSize)
	rc := C.fon_collection_get_string(
		c.ptr, cKey,
		(*C.uint8_t)(unsafe.Pointer(&buf[0])),
		C.int64_t(len(buf)),
		&e,
	)
	if err := checkRC(rc, e); err != nil {
		return "", err
	}
	// Find the null terminator.
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}


// GetIntArray reads an int32 array stored under key using the two-call pattern.
func (c *Collection) GetIntArray(key string) ([]int32, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	var actualSize C.int64_t

	// First call: query size.
	C.fon_collection_get_int_array(c.ptr, cKey, nil, 0, &actualSize, &e)
	if actualSize == 0 {
		return []int32{}, nil
	}
	buf := make([]int32, int(actualSize))
	rc := C.fon_collection_get_int_array(
		c.ptr, cKey,
		(*C.int32_t)(unsafe.Pointer(&buf[0])),
		actualSize,
		&actualSize,
		&e,
	)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	return buf, nil
}


// GetFloatArray reads a float32 array stored under key using the two-call pattern.
func (c *Collection) GetFloatArray(key string) ([]float32, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	var actualSize C.int64_t

	// First call: query size.
	C.fon_collection_get_float_array(c.ptr, cKey, nil, 0, &actualSize, &e)
	if actualSize == 0 {
		return []float32{}, nil
	}
	buf := make([]float32, int(actualSize))
	rc := C.fon_collection_get_float_array(
		c.ptr, cKey,
		(*C.float)(unsafe.Pointer(&buf[0])),
		actualSize,
		&actualSize,
		&e,
	)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	return buf, nil
}


// GetCollection returns a BORROWED handle to a nested collection under key.
// The caller MUST NOT close this handle.
func (c *Collection) GetCollection(key string) (*Collection, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var e C.FonError
	p := C.fon_collection_get_collection(c.ptr, cKey, &e)
	if p == nil {
		return nil, fonError(e)
	}
	return &Collection{ptr: p, owned: false}, nil
}


// SerializeToBytes serializes the collection to a UTF-8 byte slice using the
// two-call buffer pattern. The result does not have a trailing newline.
func (c *Collection) SerializeToBytes() ([]byte, error) {
	var e C.FonError
	var requiredSize C.int64_t

	// First call: measure.
	rc := C.fon_serialize_collection_to_buffer(c.ptr, nil, 0, &requiredSize, &e)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	if requiredSize == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, int(requiredSize))
	rc = C.fon_serialize_collection_to_buffer(
		c.ptr,
		(*C.uint8_t)(unsafe.Pointer(&buf[0])),
		requiredSize,
		&requiredSize,
		&e,
	)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	return buf, nil
}


// =============================================================================
// Dump
// =============================================================================

// Dump is an opaque handle to a FON dump (id → Collection map).
//
// A Dump returned by NewDump or DeserializeDumpFromBytes is owned by the caller
// and must be freed via Close.
type Dump struct {
	ptr unsafe.Pointer
}


// NewDump allocates a new, empty Dump.
func NewDump() *Dump {
	p := C.fon_dump_create()
	d := &Dump{ptr: p}
	runtime.SetFinalizer(d, (*Dump).finalizer)
	return d
}


// Close frees the underlying native Dump.
func (d *Dump) Close() {
	if d.ptr != nil {
		C.fon_dump_free(d.ptr)
		d.ptr = nil
	}
	runtime.SetFinalizer(d, nil)
}


func (d *Dump) finalizer() {
	d.Close()
}


// Size returns the number of records in the dump.
func (d *Dump) Size() int64 {
	return int64(C.fon_dump_size(d.ptr))
}


// Add transfers ownership of collection into the dump under id.
//
// OWNERSHIP: on success, collection is owned by the dump. The caller MUST NOT
// call collection.Close() and MUST NOT pass collection to any other function.
func (d *Dump) Add(id uint64, collection *Collection) error {
	var e C.FonError
	rc := C.fon_dump_add(d.ptr, C.uint64_t(id), collection.ptr, &e)
	if rc == C.FON_OK {
		// Ownership transferred: prevent the finalizer from double-freeing.
		collection.owned = false
		runtime.SetFinalizer(collection, nil)
	}
	return checkRC(rc, e)
}


// Get returns a BORROWED handle to the collection at index.
// The caller MUST NOT close this handle.
func (d *Dump) Get(index uint64) (*Collection, error) {
	p := C.fon_dump_get(d.ptr, C.uint64_t(index))
	if p == nil {
		return nil, fmt.Errorf("fon: index %d out of range", index)
	}
	return &Collection{ptr: p, owned: false}, nil
}


// SerializeToBytes serializes the entire dump to UTF-8 bytes (one record per
// line) using the two-call buffer pattern. maxThreads==0 uses the global Rayon
// thread pool.
func (d *Dump) SerializeToBytes(maxThreads int) ([]byte, error) {
	var e C.FonError
	var requiredSize C.int64_t

	// First call: measure.
	rc := C.fon_serialize_dump_to_buffer(d.ptr, nil, 0, &requiredSize, C.int32_t(maxThreads), &e)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	if requiredSize == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, int(requiredSize))
	rc = C.fon_serialize_dump_to_buffer(
		d.ptr,
		(*C.uint8_t)(unsafe.Pointer(&buf[0])),
		requiredSize,
		&requiredSize,
		C.int32_t(maxThreads),
		&e,
	)
	if err := checkRC(rc, e); err != nil {
		return nil, err
	}
	return buf, nil
}


// SerializeToFile writes the dump to a .fon file at path.
// maxThreads==0 uses the global Rayon thread pool.
func (d *Dump) SerializeToFile(path string, maxThreads int) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var e C.FonError
	rc := C.fon_serialize_to_file(d.ptr, cPath, C.int32_t(maxThreads), &e)
	return checkRC(rc, e)
}


// =============================================================================
// Package-level deserialization helpers
// =============================================================================

// DeserializeDumpFromBytes parses a multi-line UTF-8 buffer into a new Dump.
// The returned Dump is owned by the caller and must be closed via Dump.Close.
// maxThreads==0 uses the global Rayon thread pool.
func DeserializeDumpFromBytes(data []byte, maxThreads int) (*Dump, error) {
	var e C.FonError
	var ptr unsafe.Pointer
	if len(data) == 0 {
		ptr = C.fon_deserialize_dump_from_buffer(nil, 0, C.int32_t(maxThreads), &e)
	} else {
		ptr = C.fon_deserialize_dump_from_buffer(
			(*C.uint8_t)(unsafe.Pointer(&data[0])),
			C.int64_t(len(data)),
			C.int32_t(maxThreads),
			&e,
		)
	}
	if ptr == nil {
		return nil, fonError(e)
	}
	d := &Dump{ptr: ptr}
	runtime.SetFinalizer(d, (*Dump).finalizer)
	return d, nil
}


// DeserializeCollectionFromBytes parses a single FON record line into a new
// Collection. The returned Collection is owned by the caller and must be closed
// via Collection.Close (unless ownership is transferred via Dump.Add or
// Collection.AddCollection).
func DeserializeCollectionFromBytes(data []byte) (*Collection, error) {
	var e C.FonError
	var ptr unsafe.Pointer
	if len(data) == 0 {
		ptr = C.fon_deserialize_collection_from_buffer(nil, 0, &e)
	} else {
		ptr = C.fon_deserialize_collection_from_buffer(
			(*C.uint8_t)(unsafe.Pointer(&data[0])),
			C.int64_t(len(data)),
			&e,
		)
	}
	if ptr == nil {
		return nil, fonError(e)
	}
	c := &Collection{ptr: ptr, owned: true}
	runtime.SetFinalizer(c, (*Collection).finalizer)
	return c, nil
}


// DeserializeDumpFromFile reads a .fon file and returns a new Dump owned by
// the caller. maxThreads==0 uses the global Rayon thread pool.
func DeserializeDumpFromFile(path string, maxThreads int) (*Dump, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var e C.FonError
	ptr := C.fon_deserialize_from_file(cPath, C.int32_t(maxThreads), &e)
	if ptr == nil {
		return nil, fonError(e)
	}
	d := &Dump{ptr: ptr}
	runtime.SetFinalizer(d, (*Dump).finalizer)
	return d, nil
}
