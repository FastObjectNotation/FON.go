package fon

/*
#include "fon.h"
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Add methods store a value under a key that must not exist yet: they return an
// error matching ErrAlreadyExists when it does. Set methods store a value under
// any key, replacing an existing value of any type. Get methods read a value of
// one type: an absent key returns an error matching ErrNotFound and a value of
// another type returns one matching ErrTypeMismatch.

// AddByte stores an unsigned 8-bit integer under key.
func (c *Collection) AddByte(key string, value uint8) error {
	return c.put(key, C.FON_PUT_ADD, bytePut(value))
}

// SetByte stores an unsigned 8-bit integer under key, replacing any value.
func (c *Collection) SetByte(key string, value uint8) error {
	return c.put(key, C.FON_PUT_UPSERT, bytePut(value))
}

// GetByte reads an unsigned 8-bit integer stored under key.
func (c *Collection) GetByte(key string) (uint8, error) {
	var value C.uint8_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_byte(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return uint8(value), nil
}

// AddShort stores a 16-bit integer under key.
func (c *Collection) AddShort(key string, value int16) error {
	return c.put(key, C.FON_PUT_ADD, shortPut(value))
}

// SetShort stores a 16-bit integer under key, replacing any value.
func (c *Collection) SetShort(key string, value int16) error {
	return c.put(key, C.FON_PUT_UPSERT, shortPut(value))
}

// GetShort reads a 16-bit integer stored under key.
func (c *Collection) GetShort(key string) (int16, error) {
	var value C.int16_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_short(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return int16(value), nil
}

// AddInt stores a 32-bit integer under key.
func (c *Collection) AddInt(key string, value int32) error {
	return c.put(key, C.FON_PUT_ADD, intPut(value))
}

// SetInt stores a 32-bit integer under key, replacing any value.
func (c *Collection) SetInt(key string, value int32) error {
	return c.put(key, C.FON_PUT_UPSERT, intPut(value))
}

// GetInt reads a 32-bit integer stored under key.
func (c *Collection) GetInt(key string) (int32, error) {
	var value C.int32_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_int(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return int32(value), nil
}

// AddUInt stores an unsigned 32-bit integer under key.
func (c *Collection) AddUInt(key string, value uint32) error {
	return c.put(key, C.FON_PUT_ADD, uintPut(value))
}

// SetUInt stores an unsigned 32-bit integer under key, replacing any value.
func (c *Collection) SetUInt(key string, value uint32) error {
	return c.put(key, C.FON_PUT_UPSERT, uintPut(value))
}

// GetUInt reads an unsigned 32-bit integer stored under key.
func (c *Collection) GetUInt(key string) (uint32, error) {
	var value C.uint32_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_uint(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}

// AddLong stores a 64-bit integer under key.
func (c *Collection) AddLong(key string, value int64) error {
	return c.put(key, C.FON_PUT_ADD, longPut(value))
}

// SetLong stores a 64-bit integer under key, replacing any value.
func (c *Collection) SetLong(key string, value int64) error {
	return c.put(key, C.FON_PUT_UPSERT, longPut(value))
}

// GetLong reads a 64-bit integer stored under key.
func (c *Collection) GetLong(key string) (int64, error) {
	var value C.int64_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_long(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return int64(value), nil
}

// AddULong stores an unsigned 64-bit integer under key.
func (c *Collection) AddULong(key string, value uint64) error {
	return c.put(key, C.FON_PUT_ADD, ulongPut(value))
}

// SetULong stores an unsigned 64-bit integer under key, replacing any value.
func (c *Collection) SetULong(key string, value uint64) error {
	return c.put(key, C.FON_PUT_UPSERT, ulongPut(value))
}

// GetULong reads an unsigned 64-bit integer stored under key.
func (c *Collection) GetULong(key string) (uint64, error) {
	var value C.uint64_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_ulong(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return uint64(value), nil
}

// AddFloat stores a 32-bit float under key. NaN and infinities are stored as is.
func (c *Collection) AddFloat(key string, value float32) error {
	return c.put(key, C.FON_PUT_ADD, floatPut(value))
}

// SetFloat stores a 32-bit float under key, replacing any value.
func (c *Collection) SetFloat(key string, value float32) error {
	return c.put(key, C.FON_PUT_UPSERT, floatPut(value))
}

// GetFloat reads a 32-bit float stored under key.
func (c *Collection) GetFloat(key string) (float32, error) {
	var value C.float
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_float(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return float32(value), nil
}

// AddDouble stores a 64-bit float under key. NaN and infinities are stored as is.
func (c *Collection) AddDouble(key string, value float64) error {
	return c.put(key, C.FON_PUT_ADD, doublePut(value))
}

// SetDouble stores a 64-bit float under key, replacing any value.
func (c *Collection) SetDouble(key string, value float64) error {
	return c.put(key, C.FON_PUT_UPSERT, doublePut(value))
}

// GetDouble reads a 64-bit float stored under key.
func (c *Collection) GetDouble(key string) (float64, error) {
	var value C.double
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_double(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return 0, err
	}
	return float64(value), nil
}

// AddBool stores a boolean under key.
func (c *Collection) AddBool(key string, value bool) error {
	return c.put(key, C.FON_PUT_ADD, boolPut(value))
}

// SetBool stores a boolean under key, replacing any value.
func (c *Collection) SetBool(key string, value bool) error {
	return c.put(key, C.FON_PUT_UPSERT, boolPut(value))
}

// GetBool reads a boolean stored under key.
func (c *Collection) GetBool(key string) (bool, error) {
	var value C.int32_t
	err := c.get(key, func(b nativeKey, e *C.FonError) C.int32_t {
		return C.fon_collection_get_bool(b.handle, b.key, b.keyLen, &value, e)
	})
	if err != nil {
		return false, err
	}
	return value != 0, nil
}

// AddString stores a UTF-8 string under key. The value is stored byte for byte:
// it may be any length and may contain U+0000.
func (c *Collection) AddString(key string, value string) error {
	return c.put(key, C.FON_PUT_ADD, stringPut(value))
}

// SetString stores a UTF-8 string under key, replacing any value.
func (c *Collection) SetString(key string, value string) error {
	return c.put(key, C.FON_PUT_UPSERT, stringPut(value))
}

// GetString reads a UTF-8 string stored under key. It reads the exact byte length
// reported by the native library, so the result is never truncated.
func (c *Collection) GetString(key string) (string, error) {
	data, err := getSized[byte](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_string(b.handle, b.key, b.keyLen, (*C.uint8_t)(buf), capacity, actual, e)
	})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AddRaw stores a binary blob under key.
func (c *Collection) AddRaw(key string, data []byte) error {
	return c.put(key, C.FON_PUT_ADD, rawPut(data))
}

// SetRaw stores a binary blob under key, replacing any value.
func (c *Collection) SetRaw(key string, data []byte) error {
	return c.put(key, C.FON_PUT_UPSERT, rawPut(data))
}

// GetRaw reads a binary blob stored under key.
func (c *Collection) GetRaw(key string) ([]byte, error) {
	return getSized[byte](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_raw(b.handle, b.key, b.keyLen, (*C.uint8_t)(buf), capacity, actual, e)
	})
}

// AddByteArray stores an array of unsigned 8-bit integers under key.
func (c *Collection) AddByteArray(key string, values []uint8) error {
	return c.put(key, C.FON_PUT_ADD, byteArrayPut(values))
}

// SetByteArray stores an array of unsigned 8-bit integers under key, replacing any value.
func (c *Collection) SetByteArray(key string, values []uint8) error {
	return c.put(key, C.FON_PUT_UPSERT, byteArrayPut(values))
}

// GetByteArray reads an array of unsigned 8-bit integers stored under key.
func (c *Collection) GetByteArray(key string) ([]uint8, error) {
	return getSized[uint8](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_byte_array(b.handle, b.key, b.keyLen, (*C.uint8_t)(buf), capacity, actual, e)
	})
}

// AddShortArray stores an array of 16-bit integers under key.
func (c *Collection) AddShortArray(key string, values []int16) error {
	return c.put(key, C.FON_PUT_ADD, shortArrayPut(values))
}

// SetShortArray stores an array of 16-bit integers under key, replacing any value.
func (c *Collection) SetShortArray(key string, values []int16) error {
	return c.put(key, C.FON_PUT_UPSERT, shortArrayPut(values))
}

// GetShortArray reads an array of 16-bit integers stored under key.
func (c *Collection) GetShortArray(key string) ([]int16, error) {
	return getSized[int16](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_short_array(b.handle, b.key, b.keyLen, (*C.int16_t)(buf), capacity, actual, e)
	})
}

// AddIntArray stores an array of 32-bit integers under key.
func (c *Collection) AddIntArray(key string, values []int32) error {
	return c.put(key, C.FON_PUT_ADD, intArrayPut(values))
}

// SetIntArray stores an array of 32-bit integers under key, replacing any value.
func (c *Collection) SetIntArray(key string, values []int32) error {
	return c.put(key, C.FON_PUT_UPSERT, intArrayPut(values))
}

// GetIntArray reads an array of 32-bit integers stored under key.
func (c *Collection) GetIntArray(key string) ([]int32, error) {
	return getSized[int32](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_int_array(b.handle, b.key, b.keyLen, (*C.int32_t)(buf), capacity, actual, e)
	})
}

// AddUIntArray stores an array of unsigned 32-bit integers under key.
func (c *Collection) AddUIntArray(key string, values []uint32) error {
	return c.put(key, C.FON_PUT_ADD, uintArrayPut(values))
}

// SetUIntArray stores an array of unsigned 32-bit integers under key, replacing any value.
func (c *Collection) SetUIntArray(key string, values []uint32) error {
	return c.put(key, C.FON_PUT_UPSERT, uintArrayPut(values))
}

// GetUIntArray reads an array of unsigned 32-bit integers stored under key.
func (c *Collection) GetUIntArray(key string) ([]uint32, error) {
	return getSized[uint32](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_uint_array(b.handle, b.key, b.keyLen, (*C.uint32_t)(buf), capacity, actual, e)
	})
}

// AddLongArray stores an array of 64-bit integers under key.
func (c *Collection) AddLongArray(key string, values []int64) error {
	return c.put(key, C.FON_PUT_ADD, longArrayPut(values))
}

// SetLongArray stores an array of 64-bit integers under key, replacing any value.
func (c *Collection) SetLongArray(key string, values []int64) error {
	return c.put(key, C.FON_PUT_UPSERT, longArrayPut(values))
}

// GetLongArray reads an array of 64-bit integers stored under key.
func (c *Collection) GetLongArray(key string) ([]int64, error) {
	return getSized[int64](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_long_array(b.handle, b.key, b.keyLen, (*C.int64_t)(buf), capacity, actual, e)
	})
}

// AddULongArray stores an array of unsigned 64-bit integers under key.
func (c *Collection) AddULongArray(key string, values []uint64) error {
	return c.put(key, C.FON_PUT_ADD, ulongArrayPut(values))
}

// SetULongArray stores an array of unsigned 64-bit integers under key, replacing any value.
func (c *Collection) SetULongArray(key string, values []uint64) error {
	return c.put(key, C.FON_PUT_UPSERT, ulongArrayPut(values))
}

// GetULongArray reads an array of unsigned 64-bit integers stored under key.
func (c *Collection) GetULongArray(key string) ([]uint64, error) {
	return getSized[uint64](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_ulong_array(b.handle, b.key, b.keyLen, (*C.uint64_t)(buf), capacity, actual, e)
	})
}

// AddFloatArray stores an array of 32-bit floats under key.
func (c *Collection) AddFloatArray(key string, values []float32) error {
	return c.put(key, C.FON_PUT_ADD, floatArrayPut(values))
}

// SetFloatArray stores an array of 32-bit floats under key, replacing any value.
func (c *Collection) SetFloatArray(key string, values []float32) error {
	return c.put(key, C.FON_PUT_UPSERT, floatArrayPut(values))
}

// GetFloatArray reads an array of 32-bit floats stored under key.
func (c *Collection) GetFloatArray(key string) ([]float32, error) {
	return getSized[float32](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_float_array(b.handle, b.key, b.keyLen, (*C.float)(buf), capacity, actual, e)
	})
}

// AddDoubleArray stores an array of 64-bit floats under key.
func (c *Collection) AddDoubleArray(key string, values []float64) error {
	return c.put(key, C.FON_PUT_ADD, doubleArrayPut(values))
}

// SetDoubleArray stores an array of 64-bit floats under key, replacing any value.
func (c *Collection) SetDoubleArray(key string, values []float64) error {
	return c.put(key, C.FON_PUT_UPSERT, doubleArrayPut(values))
}

// GetDoubleArray reads an array of 64-bit floats stored under key.
func (c *Collection) GetDoubleArray(key string) ([]float64, error) {
	return getSized[float64](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_double_array(b.handle, b.key, b.keyLen, (*C.double)(buf), capacity, actual, e)
	})
}

// AddBoolArray stores an array of booleans under key.
func (c *Collection) AddBoolArray(key string, values []bool) error {
	return c.put(key, C.FON_PUT_ADD, boolArrayPut(values))
}

// SetBoolArray stores an array of booleans under key, replacing any value.
func (c *Collection) SetBoolArray(key string, values []bool) error {
	return c.put(key, C.FON_PUT_UPSERT, boolArrayPut(values))
}

// GetBoolArray reads an array of booleans stored under key.
func (c *Collection) GetBoolArray(key string) ([]bool, error) {
	raw, err := getSized[uint8](c, key, func(b nativeKey, buf unsafe.Pointer, capacity C.int64_t, actual *C.int64_t, e *C.FonError) C.int32_t {
		return C.fon_collection_get_bool_array(b.handle, b.key, b.keyLen, (*C.uint8_t)(buf), capacity, actual, e)
	})
	if err != nil {
		return nil, err
	}
	values := make([]bool, len(raw))
	for index, item := range raw {
		values[index] = item != 0
	}
	return values, nil
}

// AddStringArray stores an array of UTF-8 strings under key. Each item is stored
// byte for byte, so items may be any length and may contain U+0000.
func (c *Collection) AddStringArray(key string, values []string) error {
	return c.put(key, C.FON_PUT_ADD, stringArrayPut(values))
}

// SetStringArray stores an array of UTF-8 strings under key, replacing any value.
func (c *Collection) SetStringArray(key string, values []string) error {
	return c.put(key, C.FON_PUT_UPSERT, stringArrayPut(values))
}

// GetStringArray reads an array of UTF-8 strings stored under key in one native call.
func (c *Collection) GetStringArray(key string) ([]string, error) {
	b, err := c.bind(key)
	if err != nil {
		return nil, err
	}
	values, _, err := readMulti(func(buf multiBuffers, e *C.FonError) C.int32_t {
		return C.fon_collection_get_string_array(b.handle, b.key, b.keyLen, buf.lengths, buf.lengthsCap, buf.bytes, buf.bytesCap, buf.count, buf.total, e)
	})
	runtime.KeepAlive(c)
	return values, err
}

// bytePut builds the native put of an unsigned 8-bit integer.
func bytePut(value uint8) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_byte(b.handle, b.key, b.keyLen, C.uint8_t(value), mode, e)
	}
}

// shortPut builds the native put of a 16-bit integer.
func shortPut(value int16) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_short(b.handle, b.key, b.keyLen, C.int16_t(value), mode, e)
	}
}

// intPut builds the native put of a 32-bit integer.
func intPut(value int32) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_int(b.handle, b.key, b.keyLen, C.int32_t(value), mode, e)
	}
}

// uintPut builds the native put of an unsigned 32-bit integer.
func uintPut(value uint32) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_uint(b.handle, b.key, b.keyLen, C.uint32_t(value), mode, e)
	}
}

// longPut builds the native put of a 64-bit integer.
func longPut(value int64) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_long(b.handle, b.key, b.keyLen, C.int64_t(value), mode, e)
	}
}

// ulongPut builds the native put of an unsigned 64-bit integer.
func ulongPut(value uint64) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_ulong(b.handle, b.key, b.keyLen, C.uint64_t(value), mode, e)
	}
}

// floatPut builds the native put of a 32-bit float.
func floatPut(value float32) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_float(b.handle, b.key, b.keyLen, C.float(value), mode, e)
	}
}

// doublePut builds the native put of a 64-bit float.
func doublePut(value float64) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_double(b.handle, b.key, b.keyLen, C.double(value), mode, e)
	}
}

// boolPut builds the native put of a boolean.
func boolPut(value bool) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_bool(b.handle, b.key, b.keyLen, boolToInt32(value), mode, e)
	}
}

// stringPut builds the native put of a UTF-8 string.
func stringPut(value string) putCall {
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		ptr, length := stringArg(value)
		return C.fon_collection_put_string(b.handle, b.key, b.keyLen, ptr, length, mode, e)
	}
}

// rawPut builds the native put of a binary blob.
func rawPut(data []byte) putCall {
	data = nonNil(data)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_raw(
			b.handle,
			b.key,
			b.keyLen,
			(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
			C.int64_t(len(data)),
			mode,
			e,
		)
	}
}

// byteArrayPut builds the native put of an array of unsigned 8-bit integers.
func byteArrayPut(values []uint8) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_byte_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// shortArrayPut builds the native put of an array of 16-bit integers.
func shortArrayPut(values []int16) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_short_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.int16_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// intArrayPut builds the native put of an array of 32-bit integers.
func intArrayPut(values []int32) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_int_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.int32_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// uintArrayPut builds the native put of an array of unsigned 32-bit integers.
func uintArrayPut(values []uint32) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_uint_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.uint32_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// longArrayPut builds the native put of an array of 64-bit integers.
func longArrayPut(values []int64) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_long_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.int64_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// ulongArrayPut builds the native put of an array of unsigned 64-bit integers.
func ulongArrayPut(values []uint64) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_ulong_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.uint64_t)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// floatArrayPut builds the native put of an array of 32-bit floats.
func floatArrayPut(values []float32) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_float_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.float)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// doubleArrayPut builds the native put of an array of 64-bit floats.
func doubleArrayPut(values []float64) putCall {
	values = nonNil(values)
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_double_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.double)(unsafe.Pointer(unsafe.SliceData(values))),
			C.int64_t(len(values)),
			mode,
			e,
		)
	}
}

// boolArrayPut builds the native put of an array of booleans.
func boolArrayPut(values []bool) putCall {
	raw := make([]uint8, len(values))
	for index, value := range values {
		if value {
			raw[index] = 1
		}
	}
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_bool_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(raw))),
			C.int64_t(len(raw)),
			mode,
			e,
		)
	}
}

// stringArrayPut builds the native put of an array of UTF-8 strings: the item
// lengths and the item bytes are passed back to back.
func stringArrayPut(values []string) putCall {
	lengths := make([]int64, len(values))
	data := make([]byte, 0)
	for index, value := range values {
		lengths[index] = int64(len(value))
		data = append(data, value...)
	}
	return func(b nativeKey, mode C.int32_t, e *C.FonError) C.int32_t {
		return C.fon_collection_put_string_array(
			b.handle,
			b.key,
			b.keyLen,
			(*C.int64_t)(unsafe.Pointer(unsafe.SliceData(lengths))),
			(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
			C.int64_t(len(lengths)),
			mode,
			e,
		)
	}
}

// boolToInt32 encodes a boolean as the 0 / 1 value the native library uses.
func boolToInt32(value bool) C.int32_t {
	if value {
		return 1
	}
	return 0
}
