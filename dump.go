package fon

/*
#include "fon.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// Dump is a set of records addressed by uint64 ids. Each record is an owned
// Collection moved into the dump by Add, Set or Replace.
//
// Ids are stable in memory and renumber when the dump is serialized: each line of
// the output is one record, in ascending id order.
//
// Records obtained from Get and Range are record views: they keep the dump alive,
// and they report ErrStaleView only once their record is removed or replaced.
// Nested collections obtained from a record view report ErrStaleView once any field
// of their record is changed. Views of other records are not affected by a change.
//
// A Dump is not safe for concurrent use.
type Dump struct {
	handle unsafe.Pointer

	// closed is set when the dump is released by Close or the finalizer.
	closed bool

	// version counts the mutations made through this package on this dump.
	version uint64
}

// NewDump allocates a new, empty Dump. Call Close when it is no longer needed.
func NewDump() *Dump {
	return newDump(C.fon_dump_create())
}

// DeserializeDumpFromBytes parses a multi-line UTF-8 buffer into a new Dump owned
// by the caller. maxThreads <= 0 uses all cores; a positive value caps the threads.
func DeserializeDumpFromBytes(data []byte, maxThreads int) (*Dump, error) {
	var e C.FonError
	handle := C.fon_deserialize_dump(
		(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
		C.int64_t(len(data)),
		C.int32_t(maxThreads),
		&e,
	)
	if handle == nil {
		return nil, handleError(&e)
	}
	return newDump(handle), nil
}

// DeserializeDumpFromFile reads a .fon file into a new Dump owned by the caller.
// A missing file reports ResultFileNotFound.
func DeserializeDumpFromFile(path string, maxThreads int) (*Dump, error) {
	pathPtr, pathLen := stringArg(path)
	var e C.FonError
	handle := C.fon_deserialize_from_file(pathPtr, pathLen, C.int32_t(maxThreads), &e)
	runtime.KeepAlive(path)
	if handle == nil {
		return nil, handleError(&e)
	}
	return newDump(handle), nil
}

// newDump wraps an owned native dump handle and registers the finalizer that
// releases it.
func newDump(handle unsafe.Pointer) *Dump {
	d := &Dump{handle: handle}
	runtime.SetFinalizer(d, (*Dump).finalizer)
	return d
}

// Close releases the dump. Views of its records report ErrClosed afterwards.
func (d *Dump) Close() {
	if d.closed || d.handle == nil {
		return
	}
	C.fon_dump_free(d.handle)
	d.handle = nil
	d.closed = true
	runtime.SetFinalizer(d, nil)
}

func (d *Dump) finalizer() {
	d.Close()
}

// Size returns the number of records in the dump. It returns 0 when the dump is closed.
func (d *Dump) Size() int64 {
	if d.closed {
		return 0
	}
	size := int64(C.fon_dump_size(d.handle))
	runtime.KeepAlive(d)
	return size
}

// Add moves collection into the dump under id. It fails with ErrAlreadyExists when
// id is present. On success collection is moved into the dump: the caller must not
// use it afterwards. On error the caller keeps collection.
func (d *Dump) Add(id uint64, collection *Collection) error {
	return d.putRecord(id, collection, C.FON_PUT_ADD)
}

// Set moves collection into the dump under id, replacing any record with that id.
// Ownership rules are those of Add.
func (d *Dump) Set(id uint64, collection *Collection) error {
	return d.putRecord(id, collection, C.FON_PUT_UPSERT)
}

// Replace moves collection into the dump under id, replacing the record with that
// id. It fails with ErrNotFound when id is absent. Ownership rules are those of Add.
func (d *Dump) Replace(id uint64, collection *Collection) error {
	return d.putRecord(id, collection, C.FON_PUT_SET)
}

// Remove deletes the record with id and reports whether it was present. Removing
// an absent id still counts as a mutation of the dump, so record views are
// re-validated after it.
func (d *Dump) Remove(id uint64) (bool, error) {
	if d.closed {
		return false, ErrClosed
	}
	removed := C.fon_dump_remove(d.handle, C.uint64_t(id))
	runtime.KeepAlive(d)
	d.version++
	if removed == 0 {
		return false, nil
	}
	return true, nil
}

// RemoveMany deletes every listed id that is present and returns how many records
// were removed. Absent and duplicate ids are ignored.
func (d *Dump) RemoveMany(ids []uint64) (int, error) {
	if d.closed {
		return 0, ErrClosed
	}
	if len(ids) == 0 {
		return 0, nil
	}
	removed := C.fon_dump_remove_many(
		d.handle,
		(*C.uint64_t)(unsafe.Pointer(unsafe.SliceData(ids))),
		C.int64_t(len(ids)),
	)
	runtime.KeepAlive(d)
	d.version++
	return int(removed), nil
}

// Has reports whether a record with id is present. It returns false when the dump
// is closed.
func (d *Dump) Has(id uint64) bool {
	if d.closed {
		return false
	}
	_, _, live := d.gens(id)
	return live
}

// Ids returns the ids of all records in ascending order.
func (d *Dump) Ids() ([]uint64, error) {
	if d.closed {
		return nil, ErrClosed
	}
	ids, err := sizedRead[uint64](func(buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_dump_ids(d.handle, (*C.uint64_t)(buf), capacity, actual, e)
	})
	runtime.KeepAlive(d)
	return ids, err
}

// Get returns a record view of the record with id. It fails with ErrNotFound when
// id is absent. The view is not an owned collection: it must not be closed, and
// it must not be inserted into another container (call Clone for a copy).
func (d *Dump) Get(id uint64) (*Collection, error) {
	if d.closed {
		return nil, ErrClosed
	}
	recordGen, _, live := d.gens(id)
	if !live {
		return nil, notFoundID(id)
	}
	handle := d.recordHandle(id)
	if handle == nil {
		return nil, notFoundID(id)
	}
	return &Collection{
		handle:        handle,
		view:          true,
		dump:          d,
		id:            id,
		record:        true,
		recordGen:     recordGen,
		cachedVersion: d.version,
	}, nil
}

// Range calls fn for every record in ascending id order, passing a record view.
// Enumeration stops when fn returns false. Records removed by fn before they are
// reached are skipped.
func (d *Dump) Range(fn func(id uint64, c *Collection) bool) error {
	ids, err := d.Ids()
	if err != nil {
		return err
	}
	for _, id := range ids {
		view, err := d.Get(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return err
		}
		if !fn(id, view) {
			return nil
		}
	}
	return nil
}

// SerializeToBytes serializes the dump to UTF-8 bytes, one record per line.
// maxThreads <= 0 uses all cores; a positive value caps the threads.
func (d *Dump) SerializeToBytes(maxThreads int) ([]byte, error) {
	if d.closed {
		return nil, ErrClosed
	}
	var out *C.uint8_t
	var outLen C.int64_t
	var e C.FonError
	rc := C.fon_serialize_dump(d.handle, C.int32_t(maxThreads), &out, &outLen, &e)
	runtime.KeepAlive(d)
	if err := nativeError(rc, &e); err != nil {
		return nil, err
	}
	return takeBuffer(out, outLen), nil
}

// SerializeToFile writes the dump to a .fon file at path.
// maxThreads <= 0 uses all cores; a positive value caps the threads.
func (d *Dump) SerializeToFile(path string, maxThreads int) error {
	if d.closed {
		return ErrClosed
	}
	pathPtr, pathLen := stringArg(path)
	var e C.FonError
	rc := C.fon_serialize_to_file(d.handle, pathPtr, pathLen, C.int32_t(maxThreads), &e)
	runtime.KeepAlive(d)
	return nativeError(rc, &e)
}

// putRecord moves an owned collection into the dump under id with the given mode.
func (d *Dump) putRecord(id uint64, collection *Collection, mode C.int32_t) error {
	if d.closed {
		return ErrClosed
	}
	if err := collection.checkTransferable(nil); err != nil {
		return err
	}
	var e C.FonError
	rc := C.fon_dump_put(d.handle, C.uint64_t(id), collection.handle, mode, &e)
	runtime.KeepAlive(d)
	runtime.KeepAlive(collection)
	if err := nativeError(rc, &e); err != nil {
		return err
	}
	d.version++
	collection.transfer()
	return nil
}

// gens returns the record and tree generations of id and whether the record is
// present.
func (d *Dump) gens(id uint64) (uint32, uint32, bool) {
	var recordGen C.uint32_t
	var treeGen C.uint32_t
	live := C.fon_dump_gens(d.handle, C.uint64_t(id), &recordGen, &treeGen)
	runtime.KeepAlive(d)
	return uint32(recordGen), uint32(treeGen), live == 1
}

// recordHandle returns the borrowed native handle of record id, or nil.
func (d *Dump) recordHandle(id uint64) unsafe.Pointer {
	handle := C.fon_dump_get(d.handle, C.uint64_t(id))
	runtime.KeepAlive(d)
	return handle
}

// recordChanged records a mutation inside the tree of record id.
func (d *Dump) recordChanged(id uint64) {
	C.fon_dump_touch(d.handle, C.uint64_t(id))
	runtime.KeepAlive(d)
	d.version++
}

// notFoundID reports an absent record id.
func notFoundID(id uint64) error {
	return &FonError{Code: ResultNotFound, Message: fmt.Sprintf("record %d not found", id)}
}
