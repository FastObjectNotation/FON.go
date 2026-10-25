// Package fon implements the FON (Fast Object Notation) serialization format.
//
// FON is a fast, human-readable, line-oriented format where each line is one
// record: a comma-separated list of key=type:value pairs. Values can be
// primitives (integers, floats, booleans, strings, raw blobs) or nested
// objects and arrays.
//
// Build prerequisites:
//   - A C compiler (gcc / mingw-w64 on Windows, clang on macOS/Linux)
//   - The fon_native shared library built beforehand:
//     cargo build --release --manifest-path native/Cargo.toml
//
// The package does not embed linker flags. Supply them through CGO_LDFLAGS,
// pointing at the directory that holds the library, and make sure fon_native is
// on the runtime library search path.
package fon

// fon.h is a copy of native/include/fon.h. Keep the two files identical.

/*
#include "fon.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// Result codes mirror the FON_* result constants from fon.h.
const (
	ResultOK              = C.FON_OK
	ResultFileNotFound    = C.FON_ERROR_FILE_NOT_FOUND
	ResultParseFailed     = C.FON_ERROR_PARSE_FAILED
	ResultWriteFailed     = C.FON_ERROR_WRITE_FAILED
	ResultInvalidArgument = C.FON_ERROR_INVALID_ARGUMENT
	ResultBufferTooSmall  = C.FON_ERROR_BUFFER_TOO_SMALL
	ResultNotFound        = C.FON_ERROR_NOT_FOUND
	ResultTypeMismatch    = C.FON_ERROR_TYPE_MISMATCH
	ResultAlreadyExists   = C.FON_ERROR_ALREADY_EXISTS
	ResultStaleView       = C.FON_ERROR_STALE_VIEW
)

// Sentinel errors. Errors reported by the native library wrap the sentinel that
// matches their result code, so errors.Is(err, ErrNotFound) works on them.
var (
	// ErrNotFound reports that a key or record id is absent.
	ErrNotFound = errors.New("fon: not found")

	// ErrTypeMismatch reports that a key holds a value of another type.
	ErrTypeMismatch = errors.New("fon: type mismatch")

	// ErrAlreadyExists reports that an add found the key or id already present.
	ErrAlreadyExists = errors.New("fon: already exists")

	// ErrStaleView reports that a view outlived the value it was created from.
	ErrStaleView = errors.New("fon: stale view")

	// ErrClosed reports use of a closed dump or collection, or of a collection
	// that was moved into another container.
	ErrClosed = errors.New("fon: closed")
)

// FonError is the error returned for failures reported by the native library.
// Code holds the FON_* result code and Message the native description.
type FonError struct {
	Code    int
	Message string
}

// Error returns the message reported by the native library.
func (e *FonError) Error() string {
	return e.Message
}

// Unwrap returns the sentinel error for the result code, or nil when the code
// has no sentinel.
func (e *FonError) Unwrap() error {
	switch e.Code {
	case ResultNotFound:
		return ErrNotFound
	case ResultTypeMismatch:
		return ErrTypeMismatch
	case ResultAlreadyExists:
		return ErrAlreadyExists
	case ResultStaleView:
		return ErrStaleView
	}
	return nil
}

// NativeVersion returns the version string reported by the native library (e.g. "0.4.0").
func NativeVersion() string {
	return C.GoString(C.fon_version())
}

// SetRawUnpack enables (true) or disables (false) raw-blob unpacking during
// deserialization. The setting is global within the native library.
func SetRawUnpack(enable bool) {
	var value C.int32_t
	if enable {
		value = 1
	}
	C.fon_set_raw_unpack(value)
}

// SetMaxDepth sets the maximum nesting depth for deserialization (clamped to >= 1).
// The setting is global within the native library.
func SetMaxDepth(depth int) {
	C.fon_set_max_depth(C.int32_t(depth))
}

// nativeError converts a non-OK result code and the FonError it filled into a
// Go error. It returns nil for FON_OK.
func nativeError(rc C.int32_t, e *C.FonError) error {
	if rc == C.FON_OK {
		return nil
	}
	message := C.GoString(&e.message[0])
	if message == "" {
		message = fmt.Sprintf("fon error code %d", int(rc))
	}
	return &FonError{Code: int(rc), Message: message}
}

// handleError converts the FonError filled by a function that returns a handle.
func handleError(e *C.FonError) error {
	return nativeError(e.code, e)
}

// argumentError reports an argument rejected on the Go side before the native call.
func argumentError(message string) error {
	return &FonError{Code: ResultInvalidArgument, Message: message}
}

// staleError reports a view whose target was changed or removed.
func staleError(message string) error {
	return &FonError{Code: ResultStaleView, Message: message}
}

// nativeKey is a collection handle together with an encoded key. The owning
// Collection must be kept alive across the native call.
type nativeKey struct {
	handle unsafe.Pointer
	key    *C.uint8_t
	keyLen C.int64_t
}

// sizedCall performs one call of the size-query pattern. A nil buf is a size
// query; the required element count is written to actual.
type sizedCall func(buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t

// multiBuffers carries the arguments of the multi-string pattern.
type multiBuffers struct {
	lengths    *C.int64_t
	lengthsCap C.int64_t
	bytes      *C.uint8_t
	bytesCap   C.int64_t
	kinds      *C.int32_t
	count      *C.int64_t
	total      *C.int64_t
}

// multiCall performs one call of the multi-string pattern.
type multiCall func(buf multiBuffers, e *C.FonError) C.int32_t

// maxSizeAttempts bounds the retries of a size-query read that reports
// BUFFER_TOO_SMALL between its two calls.
const maxSizeAttempts = 3

// sizedRead reads a sequence of T with the size-query pattern: one call for the
// exact element count, one call to fill the buffer.
func sizedRead[T any](call sizedCall) ([]T, error) {
	var actual C.int64_t
	var e C.FonError
	rc := call(nil, 0, &actual, &e)
	if err := nativeError(rc, &e); err != nil {
		return nil, err
	}
	for attempt := 1; ; attempt++ {
		buf := make([]T, int(actual))
		var fill C.FonError
		rc = call(unsafe.Pointer(unsafe.SliceData(buf)), C.int64_t(len(buf)), &actual, &fill)
		if rc == C.FON_ERROR_BUFFER_TOO_SMALL && attempt < maxSizeAttempts {
			continue
		}
		if err := nativeError(rc, &fill); err != nil {
			return nil, err
		}
		return buf, nil
	}
}

// readMulti reads a sequence of strings (and their kinds, when the call fills
// them) with the multi-string pattern: one call for the counts, one call to
// fill the buffers.
func readMulti(call multiCall) ([]string, []int32, error) {
	var count C.int64_t
	var total C.int64_t
	var e C.FonError
	rc := call(multiBuffers{count: &count, total: &total}, &e)
	if err := nativeError(rc, &e); err != nil {
		return nil, nil, err
	}
	for attempt := 1; ; attempt++ {
		lengths := make([]int64, int(count))
		kinds := make([]int32, int(count))
		data := make([]byte, int(total))
		var fill C.FonError
		rc = call(multiBuffers{
			lengths:    (*C.int64_t)(unsafe.Pointer(unsafe.SliceData(lengths))),
			lengthsCap: count,
			bytes:      (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
			bytesCap:   total,
			kinds:      (*C.int32_t)(unsafe.Pointer(unsafe.SliceData(kinds))),
			count:      &count,
			total:      &total,
		}, &fill)
		if rc == C.FON_ERROR_BUFFER_TOO_SMALL && attempt < maxSizeAttempts {
			continue
		}
		if err := nativeError(rc, &fill); err != nil {
			return nil, nil, err
		}
		items, err := splitItems(lengths, data)
		if err != nil {
			return nil, nil, err
		}
		return items, kinds, nil
	}
}

// splitItems cuts data into strings using the per-item byte lengths.
func splitItems(lengths []int64, data []byte) ([]string, error) {
	items := make([]string, len(lengths))
	position := 0
	for index, length := range lengths {
		end := position + int(length)
		if length < 0 || end > len(data) {
			return nil, argumentError("native library returned inconsistent item lengths")
		}
		items[index] = string(data[position:end])
		position = end
	}
	return items, nil
}

// takeBuffer copies a buffer allocated by the native library into Go memory and
// releases the native buffer.
func takeBuffer(out *C.uint8_t, outLen C.int64_t) []byte {
	data := make([]byte, int(outLen))
	if outLen > 0 {
		copy(data, unsafe.Slice((*byte)(unsafe.Pointer(out)), int(outLen)))
	}
	C.fon_buffer_free(out, outLen)
	return data
}

// stringArg encodes a Go string as the ptr+len pair the native library expects.
func stringArg(value string) (*C.uint8_t, C.int64_t) {
	return (*C.uint8_t)(unsafe.Pointer(unsafe.StringData(value))), C.int64_t(len(value))
}

// nonNil returns values, or an empty non-nil slice when values is nil, so that
// the native call always receives a valid pointer.
func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
