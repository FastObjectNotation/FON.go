package fon

/*
#include "fon.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Collection is a FON record: an ordered map from keys to typed values.
//
// A Collection is either owned or a view:
//   - An owned collection comes from NewCollection, Clone or
//     DeserializeCollectionFromBytes. It owns native memory and is released by
//     Close, or by the garbage collector once it is unreachable.
//   - A view is borrowed: it comes from Dump.Get (a record view) or from a
//     nested getter such as GetCollection or GetCollectionArray (a nested view).
//     A view never owns native memory. It keeps its owner (the dump, or the
//     owned root collection) reachable, and it is validated on every access: if
//     the value it points into was changed or removed, the access returns an
//     error matching ErrStaleView.
//
// Passing an owned collection to AddCollection, SetCollection,
// AddCollectionArray, SetCollectionArray, Dump.Add, Dump.Set or Dump.Replace
// moves it into the container on success. The passed Collection is then unusable
// and reports ErrClosed. Views cannot be inserted: call Clone first.
//
// A Collection is not safe for concurrent use.
type Collection struct {
	handle unsafe.Pointer

	// view is true for borrowed collections.
	view bool

	// closed is set when an owned collection is released by Close or the finalizer.
	closed bool

	// transferred is set when an owned collection was moved into a container.
	transferred bool

	// version and treeVersion count the mutations made through this package on
	// an owned root collection; every mutation in its tree increments both.
	version     uint64
	treeVersion uint64

	// dump is the owner of a dump-rooted view, and id the record id it lives in.
	dump *Dump
	id   uint64

	// record is true for a record view (a collection obtained from Dump.Get).
	record bool

	// recordGen and treeGen are the generations of the record captured when the
	// view was created or last refreshed. treeGen is only used by nested views.
	recordGen uint32
	treeGen   uint32

	// root is the owned root collection of a collection-rooted nested view.
	root *Collection

	// rootTreeVersion is root.treeVersion as captured by a collection-rooted view.
	rootTreeVersion uint64

	// cachedVersion is the owner version the view last validated against.
	cachedVersion uint64
}

// putCall performs one native put for the key bound in b.
type putCall func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t

// getCall performs one native get for the key bound in b.
type getCall func(b nativeKey, e *C.FonError) C.int32_t

// sizedGetCall performs one call of the size-query pattern for the key bound in b.
type sizedGetCall func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t

// NewCollection allocates a new, empty owned Collection. Call Close when it is no
// longer needed, unless it has been moved into a dump or another collection.
func NewCollection() *Collection {
	return newOwned(C.fon_collection_create())
}

// DeserializeCollectionFromBytes parses one FON record line into a new owned
// Collection.
func DeserializeCollectionFromBytes(data []byte) (*Collection, error) {
	var e C.FonError
	handle := C.fon_deserialize_collection(
		(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
		C.int64_t(len(data)),
		&e,
	)
	if handle == nil {
		return nil, handleError(&e)
	}
	return newOwned(handle), nil
}

// newOwned wraps an owned native handle and registers the finalizer that releases it.
func newOwned(handle unsafe.Pointer) *Collection {
	c := &Collection{handle: handle}
	runtime.SetFinalizer(c, (*Collection).finalizer)
	return c
}

// Close releases an owned collection. It does nothing for views, for collections
// already closed, and for collections that were moved into a container.
// Views of a closed collection report ErrClosed.
func (c *Collection) Close() {
	if c.view || c.closed || c.transferred || c.handle == nil {
		return
	}
	C.fon_collection_free(c.handle)
	c.handle = nil
	c.closed = true
	runtime.SetFinalizer(c, nil)
}

func (c *Collection) finalizer() {
	c.Close()
}

// Size returns the number of key-value pairs in the collection. It returns 0 when
// the collection is closed or stale.
func (c *Collection) Size() int64 {
	handle, err := c.resolve()
	if err != nil {
		return 0
	}
	size := int64(C.fon_collection_size(handle))
	runtime.KeepAlive(c)
	return size
}

// Clone returns a new owned deep copy of the collection. It works on owned
// collections and on views.
func (c *Collection) Clone() (*Collection, error) {
	handle, err := c.resolve()
	if err != nil {
		return nil, err
	}
	var e C.FonError
	copied := C.fon_collection_clone(handle, &e)
	runtime.KeepAlive(c)
	if copied == nil {
		return nil, handleError(&e)
	}
	return newOwned(copied), nil
}

// Keys returns the keys of the collection in insertion order.
func (c *Collection) Keys() ([]string, error) {
	handle, err := c.resolve()
	if err != nil {
		return nil, err
	}
	keys, _, err := readMulti(func(buf multiBuffers, e *C.FonError) C.int32_t {
		return C.fon_collection_keys(handle, buf.lengths, buf.lengthsCap, buf.bytes, buf.bytesCap, buf.kinds, buf.count, buf.total, e)
	})
	runtime.KeepAlive(c)
	return keys, err
}

// Kind reports the type of the value stored under key. The boolean is false when
// the key is absent or the collection is closed or stale.
func (c *Collection) Kind(key string) (Kind, bool) {
	kind, found, err := c.kindOf(key)
	if err != nil || !found {
		return Kind{}, false
	}
	return kind, true
}

// Has reports whether key is present. It returns false when the collection is
// closed or stale.
func (c *Collection) Has(key string) bool {
	_, found, err := c.kindOf(key)
	return err == nil && found
}

// Remove deletes key and reports whether it was present.
func (c *Collection) Remove(key string) (bool, error) {
	b, err := c.bind(key)
	if err != nil {
		return false, err
	}
	removed := C.fon_collection_remove(b.handle, b.key, b.keyLen)
	runtime.KeepAlive(c)
	if removed == 0 {
		return false, nil
	}
	c.afterMutation()
	return true, nil
}

// Get reads the value stored under key as a dynamic value. The dynamic type is the
// type returned by the typed getter for the stored kind: uint8, int16, int32,
// uint32, int64, uint64, float32, float64, bool, string, []byte (raw), *Collection,
// or the matching slice type for arrays ([]*Collection for collection arrays).
func (c *Collection) Get(key string) (any, error) {
	kind, found, err := c.kindOf(key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, keyNotFound(key)
	}
	switch kind.Type {
	case TypeByte:
		if kind.Array {
			return c.GetByteArray(key)
		}
		return c.GetByte(key)
	case TypeShort:
		if kind.Array {
			return c.GetShortArray(key)
		}
		return c.GetShort(key)
	case TypeInt:
		if kind.Array {
			return c.GetIntArray(key)
		}
		return c.GetInt(key)
	case TypeUInt:
		if kind.Array {
			return c.GetUIntArray(key)
		}
		return c.GetUInt(key)
	case TypeLong:
		if kind.Array {
			return c.GetLongArray(key)
		}
		return c.GetLong(key)
	case TypeULong:
		if kind.Array {
			return c.GetULongArray(key)
		}
		return c.GetULong(key)
	case TypeFloat:
		if kind.Array {
			return c.GetFloatArray(key)
		}
		return c.GetFloat(key)
	case TypeDouble:
		if kind.Array {
			return c.GetDoubleArray(key)
		}
		return c.GetDouble(key)
	case TypeBool:
		if kind.Array {
			return c.GetBoolArray(key)
		}
		return c.GetBool(key)
	case TypeString:
		if kind.Array {
			return c.GetStringArray(key)
		}
		return c.GetString(key)
	case TypeRaw:
		return c.GetRaw(key)
	case TypeObject:
		if kind.Array {
			return c.GetCollectionArray(key)
		}
		return c.GetCollection(key)
	}
	return nil, argumentError(fmt.Sprintf("unsupported value kind for key '%s'", key))
}

// SerializeToBytes serializes the collection to UTF-8 bytes. The output is one
// record without a trailing newline.
func (c *Collection) SerializeToBytes() ([]byte, error) {
	handle, err := c.resolve()
	if err != nil {
		return nil, err
	}
	var out *C.uint8_t
	var outLen C.int64_t
	var e C.FonError
	rc := C.fon_serialize_collection(handle, &out, &outLen, &e)
	runtime.KeepAlive(c)
	if err := nativeError(rc, &e); err != nil {
		return nil, err
	}
	return takeBuffer(out, outLen), nil
}

// SerializeToString serializes the collection to a string. See SerializeToBytes.
func (c *Collection) SerializeToString() (string, error) {
	data, err := c.SerializeToBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AddCollection stores child under key. On success child is moved into c: the
// caller must not use child afterwards. On error the caller keeps child.
func (c *Collection) AddCollection(key string, child *Collection) error {
	return c.putChild(key, child, C.FON_PUT_ADD)
}

// SetCollection stores child under key, replacing any existing value. Ownership
// rules are those of AddCollection.
func (c *Collection) SetCollection(key string, child *Collection) error {
	return c.putChild(key, child, C.FON_PUT_UPSERT)
}

// GetCollection returns a nested view of the collection stored under key. The
// view is invalidated by any change to the field's record or root tree. Call
// Clone on it to keep an independent copy.
func (c *Collection) GetCollection(key string) (*Collection, error) {
	b, err := c.bind(key)
	if err != nil {
		return nil, err
	}
	var e C.FonError
	handle := C.fon_collection_get_collection(b.handle, b.key, b.keyLen, &e)
	runtime.KeepAlive(c)
	if handle == nil {
		return nil, handleError(&e)
	}
	return c.viewOf(handle)
}

// AddCollectionArray stores children under key as an array. On success every child
// is moved into c. On error no child is moved and the caller keeps all of them.
func (c *Collection) AddCollectionArray(key string, children []*Collection) error {
	return c.putChildren(key, children, C.FON_PUT_ADD)
}

// SetCollectionArray stores children under key as an array, replacing any existing
// value. Ownership rules are those of AddCollectionArray.
func (c *Collection) SetCollectionArray(key string, children []*Collection) error {
	return c.putChildren(key, children, C.FON_PUT_UPSERT)
}

// GetCollectionArray returns nested views of every element of the collection array
// stored under key.
func (c *Collection) GetCollectionArray(key string) ([]*Collection, error) {
	b, err := c.bind(key)
	if err != nil {
		return nil, err
	}
	handles, err := sizedRead[unsafe.Pointer](func(buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_collection_array(b.handle, b.key, b.keyLen, (*unsafe.Pointer)(buf), capacity, actual, e)
	})
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	views := make([]*Collection, len(handles))
	for index, handle := range handles {
		view, err := c.viewOf(handle)
		if err != nil {
			return nil, err
		}
		views[index] = view
	}
	return views, nil
}

// bind resolves the collection for a key operation: it validates the receiver
// and encodes the key.
func (c *Collection) bind(key string) (nativeKey, error) {
	handle, err := c.resolve()
	if err != nil {
		return nativeKey{}, err
	}
	keyPtr, keyLen := stringArg(key)
	return nativeKey{handle: handle, key: keyPtr, keyLen: keyLen}, nil
}

// kindOf reads the kind of key, reporting whether the key is present.
func (c *Collection) kindOf(key string) (Kind, bool, error) {
	b, err := c.bind(key)
	if err != nil {
		return Kind{}, false, err
	}
	code := C.fon_collection_kind_of(b.handle, b.key, b.keyLen)
	runtime.KeepAlive(c)
	if code < 0 {
		return Kind{}, false, nil
	}
	kind, ok := kindFromCode(int32(code))
	if !ok {
		return Kind{}, false, argumentError(fmt.Sprintf("native library returned an unknown kind for key '%s'", key))
	}
	return kind, true, nil
}

// put performs a native put of one value under key and records the mutation.
func (c *Collection) put(key string, mode C.int32_t, call putCall) error {
	b, err := c.bind(key)
	if err != nil {
		return err
	}
	var e C.FonError
	rc := call(b, mode, &e)
	runtime.KeepAlive(c)
	if err := nativeError(rc, &e); err != nil {
		return err
	}
	c.afterMutation()
	return nil
}

// get performs a native get of one fixed-size value under key.
func (c *Collection) get(key string, call getCall) error {
	b, err := c.bind(key)
	if err != nil {
		return err
	}
	var e C.FonError
	rc := call(b, &e)
	runtime.KeepAlive(c)
	return nativeError(rc, &e)
}

// getSized reads a variable-length value under key with the size-query pattern.
func getSized[T any](c *Collection, key string, call sizedGetCall) ([]T, error) {
	b, err := c.bind(key)
	if err != nil {
		return nil, err
	}
	values, err := sizedRead[T](func(buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return call(b, buf, capacity, actual, e)
	})
	runtime.KeepAlive(c)
	return values, err
}

// putChild moves an owned child into c under key.
func (c *Collection) putChild(key string, child *Collection, mode C.int32_t) error {
	b, err := c.bind(key)
	if err != nil {
		return err
	}
	if err := child.checkTransferable(c); err != nil {
		return err
	}
	var e C.FonError
	rc := C.fon_collection_put_collection(b.handle, b.key, b.keyLen, child.handle, mode, &e)
	runtime.KeepAlive(c)
	runtime.KeepAlive(child)
	if err := nativeError(rc, &e); err != nil {
		return err
	}
	c.afterMutation()
	child.transfer()
	return nil
}

// putChildren moves every owned child into c under key as an array.
func (c *Collection) putChildren(key string, children []*Collection, mode C.int32_t) error {
	b, err := c.bind(key)
	if err != nil {
		return err
	}
	if err := checkChildren(c, children); err != nil {
		return err
	}
	handles := make([]unsafe.Pointer, len(children))
	for index, child := range children {
		handles[index] = child.handle
	}
	var e C.FonError
	rc := C.fon_collection_put_collection_array(
		b.handle,
		b.key,
		b.keyLen,
		unsafe.SliceData(handles),
		C.int64_t(len(handles)),
		mode,
		&e,
	)
	runtime.KeepAlive(c)
	runtime.KeepAlive(children)
	if err := nativeError(rc, &e); err != nil {
		return err
	}
	c.afterMutation()
	for _, child := range children {
		child.transfer()
	}
	return nil
}

// checkTransferable reports whether c may be moved into target (nil for a dump).
func (c *Collection) checkTransferable(target *Collection) error {
	if c == nil {
		return argumentError("collection is nil")
	}
	if c.view {
		return argumentError("collection is a view; call Clone() before inserting it")
	}
	if c.closed || c.transferred || c.handle == nil {
		return ErrClosed
	}
	if target == c {
		return argumentError("a collection cannot be inserted into itself")
	}
	if target != nil && target.view && target.dump == nil && target.root == c {
		return argumentError("a collection cannot be inserted into its own nested view")
	}
	return nil
}

// checkChildren validates every child of an array put before the native call.
func checkChildren(target *Collection, children []*Collection) error {
	seen := make(map[*Collection]struct{}, len(children))
	for _, child := range children {
		if err := child.checkTransferable(target); err != nil {
			return err
		}
		if _, duplicate := seen[child]; duplicate {
			return argumentError("the same collection appears more than once in the array")
		}
		seen[child] = struct{}{}
	}
	return nil
}

// transfer marks an owned collection as moved into a container. The native
// memory now belongs to the container, so the finalizer must not release it.
func (c *Collection) transfer() {
	c.transferred = true
	c.handle = nil
	runtime.SetFinalizer(c, nil)
}

// viewOf wraps a borrowed handle returned by a nested getter of c.
func (c *Collection) viewOf(handle unsafe.Pointer) (*Collection, error) {
	if c.dump != nil {
		recordGen, treeGen, live := c.dump.gens(c.id)
		if !live {
			return nil, staleError(fmt.Sprintf("record %d is no longer present", c.id))
		}
		return &Collection{
			handle:        handle,
			view:          true,
			dump:          c.dump,
			id:            c.id,
			recordGen:     recordGen,
			treeGen:       treeGen,
			cachedVersion: c.dump.version,
		}, nil
	}
	root := c
	if c.view {
		root = c.root
	}
	return &Collection{
		handle:          handle,
		view:            true,
		root:            root,
		rootTreeVersion: root.treeVersion,
		cachedVersion:   root.version,
	}, nil
}

// resolve validates the collection and returns its native handle for a read or a
// write. Views re-check their owner and, when it changed, their record or tree.
func (c *Collection) resolve() (unsafe.Pointer, error) {
	if !c.view {
		if c.closed || c.transferred || c.handle == nil {
			return nil, ErrClosed
		}
		return c.handle, nil
	}
	if c.dump != nil {
		return c.resolveDump()
	}
	return c.resolveRoot()
}

// resolveDump validates a dump-rooted view.
func (c *Collection) resolveDump() (unsafe.Pointer, error) {
	d := c.dump
	if d.closed {
		return nil, ErrClosed
	}
	if d.version == c.cachedVersion {
		return c.handle, nil
	}
	recordGen, treeGen, live := d.gens(c.id)
	if !live || recordGen != c.recordGen {
		return nil, staleError(fmt.Sprintf("record %d was removed or replaced", c.id))
	}
	if c.record {
		c.handle = d.recordHandle(c.id)
	} else if treeGen != c.treeGen {
		return nil, staleError(fmt.Sprintf("record %d was changed after this view was taken", c.id))
	}
	c.cachedVersion = d.version
	return c.handle, nil
}

// resolveRoot validates a collection-rooted nested view.
func (c *Collection) resolveRoot() (unsafe.Pointer, error) {
	root := c.root
	if root.closed {
		return nil, ErrClosed
	}
	if root.transferred {
		return nil, staleError("the owning collection was moved into another container")
	}
	if root.version == c.cachedVersion {
		return c.handle, nil
	}
	if root.treeVersion != c.rootTreeVersion {
		return nil, staleError("the owning collection was changed after this view was taken")
	}
	c.cachedVersion = root.version
	return c.handle, nil
}

// afterMutation records a successful mutation made through c. The mutated
// target's owner is updated, so that views of other records or of the same tree
// are invalidated as the view rules require, while c itself stays valid.
func (c *Collection) afterMutation() {
	if !c.view {
		c.version++
		c.treeVersion++
		return
	}
	if c.dump == nil {
		root := c.root
		root.version++
		root.treeVersion++
		c.rootTreeVersion = root.treeVersion
		c.cachedVersion = root.version
		return
	}
	c.dump.recordChanged(c.id)
	c.cachedVersion = c.dump.version
	if c.record {
		c.handle = c.dump.recordHandle(c.id)
		return
	}
	recordGen, treeGen, _ := c.dump.gens(c.id)
	c.recordGen = recordGen
	c.treeGen = treeGen
}

// keyNotFound reports an absent key found by the dynamic Get.
func keyNotFound(key string) error {
	return &FonError{Code: ResultNotFound, Message: fmt.Sprintf("key '%s' not found", key)}
}
