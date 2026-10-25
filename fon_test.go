package fon_test

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	fon "github.com/FastObjectNotation/FON.go"
)

// TestNativeVersion asserts the library reports a well-formed semver string.
// It deliberately does not pin an exact version, so routine version bumps
// don't break the test.
func TestNativeVersion(t *testing.T) {
	got := fon.NativeVersion()
	parts := strings.Split(got, ".")
	if len(parts) != 3 {
		t.Fatalf("NativeVersion() = %q, want a semver-like X.Y.Z string", got)
	}
	for _, p := range parts {
		if p == "" {
			t.Fatalf("NativeVersion() = %q has an empty component", got)
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				t.Fatalf("NativeVersion() = %q has a non-numeric component %q", got, p)
			}
		}
	}
}

// TestRoundtrip builds a collection (id, name, price), adds it to a dump,
// serializes to a UTF-8 buffer, deserializes back, and asserts field equality.
func TestRoundtrip(t *testing.T) {
	// Build the collection.
	c := fon.NewCollection()
	defer c.Close()

	if err := c.AddInt("id", 42); err != nil {
		t.Fatalf("AddInt: %v", err)
	}
	if err := c.AddString("name", "Test Item"); err != nil {
		t.Fatalf("AddString: %v", err)
	}
	if err := c.AddDouble("price", 99.99); err != nil {
		t.Fatalf("AddDouble: %v", err)
	}

	// Wrap it in a Dump.
	dump := fon.NewDump()
	defer dump.Close()

	// Dump.Add takes ownership of cOwned; we must not Close it after this call.
	// We wrap in a dedicated variable so that the deferred Close on the original
	// does not run (we nil-out by transferring to the dump).
	cOwned := fon.NewCollection()
	if err := cOwned.AddInt("id", 42); err != nil {
		t.Fatalf("AddInt (owned): %v", err)
	}
	if err := cOwned.AddString("name", "Test Item"); err != nil {
		t.Fatalf("AddString (owned): %v", err)
	}
	if err := cOwned.AddDouble("price", 99.99); err != nil {
		t.Fatalf("AddDouble (owned): %v", err)
	}
	if err := dump.Add(0, cOwned); err != nil {
		t.Fatalf("Dump.Add: %v", err)
	}
	// cOwned is now owned by dump — no Close needed.

	if dump.Size() != 1 {
		t.Fatalf("Dump.Size() = %d, want 1", dump.Size())
	}

	// Serialize to bytes.
	serialized, err := dump.SerializeToBytes(0)
	if err != nil {
		t.Fatalf("SerializeToBytes: %v", err)
	}
	if len(serialized) == 0 {
		t.Fatal("serialized output is empty")
	}
	t.Logf("serialized: %s", serialized)

	// Verify the FON wire format contains expected tokens.
	line := strings.TrimSpace(string(serialized))
	if !strings.Contains(line, "id=i:42") {
		t.Errorf("serialized does not contain 'id=i:42': %s", line)
	}
	if !strings.Contains(line, `name=s:"Test Item"`) {
		t.Errorf("serialized does not contain 'name=s:\"Test Item\"': %s", line)
	}
	if !strings.Contains(line, "price=d:") {
		t.Errorf("serialized does not contain 'price=d:': %s", line)
	}

	// Deserialize back.
	dump2, err := fon.DeserializeDumpFromBytes(serialized, 0)
	if err != nil {
		t.Fatalf("DeserializeDumpFromBytes: %v", err)
	}
	defer dump2.Close()

	if dump2.Size() != 1 {
		t.Fatalf("deserialized Dump.Size() = %d, want 1", dump2.Size())
	}

	col, err := dump2.Get(0)
	if err != nil {
		t.Fatalf("Dump.Get(0): %v", err)
	}
	// col is a view; do NOT close it.

	gotID, err := col.GetInt("id")
	if err != nil {
		t.Fatalf("GetInt(id): %v", err)
	}
	if gotID != 42 {
		t.Errorf("id = %d, want 42", gotID)
	}

	gotName, err := col.GetString("name")
	if err != nil {
		t.Fatalf("GetString(name): %v", err)
	}
	if gotName != "Test Item" {
		t.Errorf("name = %q, want %q", gotName, "Test Item")
	}

	gotPrice, err := col.GetDouble("price")
	if err != nil {
		t.Fatalf("GetDouble(price): %v", err)
	}
	if gotPrice != 99.99 {
		t.Errorf("price = %f, want 99.99", gotPrice)
	}
}

// TestCollectionSerializeRoundtrip serializes a single collection to a buffer
// and deserializes it back without going through a Dump.
func TestCollectionSerializeRoundtrip(t *testing.T) {
	src := fon.NewCollection()
	defer src.Close()

	if err := src.AddLong("ts", 1700000000); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := src.AddBool("active", true); err != nil {
		t.Fatalf("AddBool: %v", err)
	}

	data, err := src.SerializeToBytes()
	if err != nil {
		t.Fatalf("SerializeToBytes: %v", err)
	}
	t.Logf("collection serialized: %s", data)

	dst, err := fon.DeserializeCollectionFromBytes(data)
	if err != nil {
		t.Fatalf("DeserializeCollectionFromBytes: %v", err)
	}
	defer dst.Close()

	ts, err := dst.GetLong("ts")
	if err != nil {
		t.Fatalf("GetLong: %v", err)
	}
	if ts != 1700000000 {
		t.Errorf("ts = %d, want 1700000000", ts)
	}

	active, err := dst.GetBool("active")
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if !active {
		t.Errorf("active = %v, want true", active)
	}
}

// TestCollectionArray verifies the add/get round-trip for an array of nested
// collections (AddCollectionArray / GetCollectionArray).
func TestCollectionArray(t *testing.T) {
	// Build two sub-collections.
	item0 := fon.NewCollection()
	if err := item0.AddInt("id", 1); err != nil {
		t.Fatalf("item0.AddInt: %v", err)
	}
	if err := item0.AddString("name", "Alpha"); err != nil {
		t.Fatalf("item0.AddString: %v", err)
	}

	item1 := fon.NewCollection()
	if err := item1.AddInt("id", 2); err != nil {
		t.Fatalf("item1.AddInt: %v", err)
	}
	if err := item1.AddString("name", "Beta"); err != nil {
		t.Fatalf("item1.AddString: %v", err)
	}

	// Build the parent and transfer ownership of both items.
	parent := fon.NewCollection()
	defer parent.Close()

	if err := parent.AddCollectionArray("items", []*fon.Collection{item0, item1}); err != nil {
		t.Fatalf("AddCollectionArray: %v", err)
	}
	// item0 and item1 are now owned by parent — do NOT close them.

	// Serialize the parent to bytes.
	data, err := parent.SerializeToBytes()
	if err != nil {
		t.Fatalf("SerializeToBytes: %v", err)
	}
	t.Logf("collection_array serialized: %s", data)

	// Deserialize back.
	dst, err := fon.DeserializeCollectionFromBytes(data)
	if err != nil {
		t.Fatalf("DeserializeCollectionFromBytes: %v", err)
	}
	defer dst.Close()

	// Retrieve the collection array.
	items, err := dst.GetCollectionArray("items")
	if err != nil {
		t.Fatalf("GetCollectionArray: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// items are views — do NOT close them.

	// Verify item 0.
	id0, err := items[0].GetInt("id")
	if err != nil {
		t.Fatalf("items[0].GetInt: %v", err)
	}
	if id0 != 1 {
		t.Errorf("items[0].id = %d, want 1", id0)
	}
	name0, err := items[0].GetString("name")
	if err != nil {
		t.Fatalf("items[0].GetString: %v", err)
	}
	if name0 != "Alpha" {
		t.Errorf("items[0].name = %q, want %q", name0, "Alpha")
	}

	// Verify item 1.
	id1, err := items[1].GetInt("id")
	if err != nil {
		t.Fatalf("items[1].GetInt: %v", err)
	}
	if id1 != 2 {
		t.Errorf("items[1].id = %d, want 2", id1)
	}
	name1, err := items[1].GetString("name")
	if err != nil {
		t.Fatalf("items[1].GetString: %v", err)
	}
	if name1 != "Beta" {
		t.Errorf("items[1].name = %q, want %q", name1, "Beta")
	}
}

// TestIntArray verifies add/get round-trip for integer arrays.
func TestIntArray(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()

	want := []int32{10, 20, 30, 40, 50}
	if err := c.AddIntArray("scores", want); err != nil {
		t.Fatalf("AddIntArray: %v", err)
	}

	data, err := c.SerializeToBytes()
	if err != nil {
		t.Fatalf("SerializeToBytes: %v", err)
	}

	c2, err := fon.DeserializeCollectionFromBytes(data)
	if err != nil {
		t.Fatalf("DeserializeCollectionFromBytes: %v", err)
	}
	defer c2.Close()

	got, err := c2.GetIntArray("scores")
	if err != nil {
		t.Fatalf("GetIntArray: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len(scores) = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("scores[%d] = %d, want %d", i, got[i], v)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func expectErr(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want errors.Is(%v)", err, target)
	}
}

func expectCode(t *testing.T, err error, code int) {
	t.Helper()
	var fonErr *fon.FonError
	if !errors.As(err, &fonErr) || fonErr.Code != code {
		t.Fatalf("error = %v, want a FonError with code %d", err, code)
	}
}

func expectValue[T comparable](t *testing.T, name string, got T, err error, want T) {
	t.Helper()
	mustNoErr(t, err)
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func expectText(t *testing.T, name string, got string, want string) {
	t.Helper()
	if got == want {
		return
	}
	position := 0
	for position < len(got) && position < len(want) && got[position] == want[position] {
		position++
	}
	t.Errorf("%s: got %d bytes, want %d bytes (first difference at byte %d)", name, len(got), len(want), position)
}

func expectSame(t *testing.T, name string, got any, want any) {
	t.Helper()
	gotText := fmt.Sprintf("%T %v", got, got)
	wantText := fmt.Sprintf("%T %v", want, want)
	if gotText != wantText {
		t.Errorf("%s = %s, want %s", name, gotText, wantText)
	}
}

func expectIDs(t *testing.T, got []uint64, want []uint64) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

func roundtripCollection(t *testing.T, c *fon.Collection) *fon.Collection {
	t.Helper()
	data, err := c.SerializeToBytes()
	mustNoErr(t, err)
	restored, err := fon.DeserializeCollectionFromBytes(data)
	mustNoErr(t, err)
	return restored
}

func roundtripDump(t *testing.T, d *fon.Dump) *fon.Dump {
	t.Helper()
	data, err := d.SerializeToBytes(0)
	mustNoErr(t, err)
	restored, err := fon.DeserializeDumpFromBytes(data, 0)
	mustNoErr(t, err)
	return restored
}

// makeText returns exactly size bytes of ASCII text.
func makeText(size int) string {
	pattern := "abcdefghijklmnopqrstuvwxyz0123456789"
	return strings.Repeat(pattern, size/len(pattern)+1)[:size]
}

// fillAllKinds stores one value of every kind (and the boundary values of each
// numeric type) into c.
func fillAllKinds(t *testing.T, c *fon.Collection) {
	t.Helper()
	mustNoErr(t, c.AddByte("byte_min", 0))
	mustNoErr(t, c.AddByte("byte_max", math.MaxUint8))
	mustNoErr(t, c.AddShort("short_min", math.MinInt16))
	mustNoErr(t, c.AddShort("short_max", math.MaxInt16))
	mustNoErr(t, c.AddInt("int_min", math.MinInt32))
	mustNoErr(t, c.AddInt("int_max", math.MaxInt32))
	mustNoErr(t, c.AddUInt("uint_min", 0))
	mustNoErr(t, c.AddUInt("uint_max", math.MaxUint32))
	mustNoErr(t, c.AddLong("long_min", math.MinInt64))
	mustNoErr(t, c.AddLong("long_max", math.MaxInt64))
	mustNoErr(t, c.AddULong("ulong_min", 0))
	mustNoErr(t, c.AddULong("ulong_max", math.MaxUint64))
	mustNoErr(t, c.AddFloat("float_nan", float32(math.NaN())))
	mustNoErr(t, c.AddFloat("float_inf", float32(math.Inf(1))))
	mustNoErr(t, c.AddFloat("float_ninf", float32(math.Inf(-1))))
	mustNoErr(t, c.AddDouble("double_nan", math.NaN()))
	mustNoErr(t, c.AddDouble("double_inf", math.Inf(1)))
	mustNoErr(t, c.AddDouble("double_ninf", math.Inf(-1)))
	mustNoErr(t, c.AddBool("bool_true", true))
	mustNoErr(t, c.AddBool("bool_false", false))
	mustNoErr(t, c.AddString("string", "hello"))
	mustNoErr(t, c.AddRaw("raw", []byte{0x00, 0x01, 0xff}))
	mustNoErr(t, c.AddByteArray("byte_array", []uint8{0, 255}))
	mustNoErr(t, c.AddShortArray("short_array", []int16{math.MinInt16, math.MaxInt16}))
	mustNoErr(t, c.AddIntArray("int_array", []int32{math.MinInt32, math.MaxInt32}))
	mustNoErr(t, c.AddUIntArray("uint_array", []uint32{0, math.MaxUint32}))
	mustNoErr(t, c.AddLongArray("long_array", []int64{math.MinInt64, math.MaxInt64}))
	mustNoErr(t, c.AddULongArray("ulong_array", []uint64{0, math.MaxUint64}))
	mustNoErr(t, c.AddFloatArray("float_array", []float32{1.5, float32(math.Inf(-1))}))
	mustNoErr(t, c.AddFloatArray("float_nan_array", []float32{float32(math.NaN())}))
	mustNoErr(t, c.AddDoubleArray("double_array", []float64{-0.25, math.Inf(1)}))
	mustNoErr(t, c.AddDoubleArray("double_nan_array", []float64{math.NaN()}))
	mustNoErr(t, c.AddBoolArray("bool_array", []bool{true, false, true}))
	mustNoErr(t, c.AddStringArray("string_array", []string{"a", "", "c"}))

	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("n", 1))
	mustNoErr(t, c.AddCollection("object", child))

	first := fon.NewCollection()
	mustNoErr(t, first.AddInt("id", 1))
	second := fon.NewCollection()
	mustNoErr(t, second.AddInt("id", 2))
	mustNoErr(t, c.AddCollectionArray("object_array", []*fon.Collection{first, second}))
}

// checkAllKinds verifies the values written by fillAllKinds, through the dynamic
// Get, the typed getters and Kind.
func checkAllKinds(t *testing.T, c *fon.Collection) {
	t.Helper()
	expected := []struct {
		key  string
		want any
	}{
		{"byte_min", uint8(0)},
		{"byte_max", uint8(math.MaxUint8)},
		{"short_min", int16(math.MinInt16)},
		{"short_max", int16(math.MaxInt16)},
		{"int_min", int32(math.MinInt32)},
		{"int_max", int32(math.MaxInt32)},
		{"uint_min", uint32(0)},
		{"uint_max", uint32(math.MaxUint32)},
		{"long_min", int64(math.MinInt64)},
		{"long_max", int64(math.MaxInt64)},
		{"ulong_min", uint64(0)},
		{"ulong_max", uint64(math.MaxUint64)},
		{"float_inf", float32(math.Inf(1))},
		{"float_ninf", float32(math.Inf(-1))},
		{"double_inf", math.Inf(1)},
		{"double_ninf", math.Inf(-1)},
		{"bool_true", true},
		{"bool_false", false},
		{"string", "hello"},
		{"raw", []byte{0x00, 0x01, 0xff}},
		{"byte_array", []uint8{0, 255}},
		{"short_array", []int16{math.MinInt16, math.MaxInt16}},
		{"int_array", []int32{math.MinInt32, math.MaxInt32}},
		{"uint_array", []uint32{0, math.MaxUint32}},
		{"long_array", []int64{math.MinInt64, math.MaxInt64}},
		{"ulong_array", []uint64{0, math.MaxUint64}},
		{"float_array", []float32{1.5, float32(math.Inf(-1))}},
		{"double_array", []float64{-0.25, math.Inf(1)}},
		{"bool_array", []bool{true, false, true}},
		{"string_array", []string{"a", "", "c"}},
	}
	for _, entry := range expected {
		got, err := c.Get(entry.key)
		mustNoErr(t, err)
		expectSame(t, entry.key, got, entry.want)
	}

	nan32, err := c.GetFloat("float_nan")
	mustNoErr(t, err)
	if !math.IsNaN(float64(nan32)) {
		t.Errorf("float_nan = %v, want NaN", nan32)
	}
	nan64, err := c.GetDouble("double_nan")
	mustNoErr(t, err)
	if !math.IsNaN(nan64) {
		t.Errorf("double_nan = %v, want NaN", nan64)
	}
	nanArray32, err := c.GetFloatArray("float_nan_array")
	mustNoErr(t, err)
	if len(nanArray32) != 1 || !math.IsNaN(float64(nanArray32[0])) {
		t.Errorf("float_nan_array = %v, want [NaN]", nanArray32)
	}
	nanArray64, err := c.GetDoubleArray("double_nan_array")
	mustNoErr(t, err)
	if len(nanArray64) != 1 || !math.IsNaN(nanArray64[0]) {
		t.Errorf("double_nan_array = %v, want [NaN]", nanArray64)
	}

	kinds := []struct {
		key  string
		want fon.Kind
	}{
		{"int_array", fon.Kind{Type: fon.TypeInt, Array: true}},
		{"raw", fon.Kind{Type: fon.TypeRaw}},
		{"string", fon.Kind{Type: fon.TypeString}},
		{"object", fon.Kind{Type: fon.TypeObject}},
		{"object_array", fon.Kind{Type: fon.TypeObject, Array: true}},
	}
	for _, entry := range kinds {
		got, ok := c.Kind(entry.key)
		if !ok || got != entry.want {
			t.Errorf("Kind(%s) = %v, %v, want %v", entry.key, got, ok, entry.want)
		}
	}

	object, err := c.GetCollection("object")
	mustNoErr(t, err)
	n, err := object.GetInt("n")
	expectValue(t, "object.n", n, err, int32(1))

	items, err := c.GetCollectionArray("object_array")
	mustNoErr(t, err)
	if len(items) != 2 {
		t.Fatalf("len(object_array) = %d, want 2", len(items))
	}
	id0, err := items[0].GetInt("id")
	expectValue(t, "object_array[0].id", id0, err, int32(1))
	id1, err := items[1].GetInt("id")
	expectValue(t, "object_array[1].id", id1, err, int32(2))
}

// ---------------------------------------------------------------------------
// Design section 6.1: round-trip of every kind
// ---------------------------------------------------------------------------

func TestRoundtripAllKinds(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	fillAllKinds(t, c)
	checkAllKinds(t, c)

	restored := roundtripCollection(t, c)
	defer restored.Close()
	checkAllKinds(t, restored)
}

func TestRoundtripAllKindsThroughDump(t *testing.T) {
	c := fon.NewCollection()
	fillAllKinds(t, c)

	d := fon.NewDump()
	defer d.Close()
	mustNoErr(t, d.Add(7, c))

	restoredDump := roundtripDump(t, d)
	defer restoredDump.Close()
	record, err := restoredDump.Get(0)
	mustNoErr(t, err)
	checkAllKinds(t, record)
}

// ---------------------------------------------------------------------------
// Design section 6.2: strings
// ---------------------------------------------------------------------------

func TestStringSizes(t *testing.T) {
	for _, size := range []int{8191, 8192, 100000} {
		value := makeText(size)
		c := fon.NewCollection()
		mustNoErr(t, c.AddString("value", value))
		restored := roundtripCollection(t, c)
		got, err := restored.GetString("value")
		mustNoErr(t, err)
		expectText(t, fmt.Sprintf("value(%d)", size), got, value)
		c.Close()
		restored.Close()
	}
}

func TestStringArrayLargeMiddleItem(t *testing.T) {
	middle := strings.Repeat("b", 8192)
	values := []string{"x", middle, "y"}

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddStringArray("items", values))

	restored := roundtripCollection(t, c)
	defer restored.Close()
	got, err := restored.GetStringArray("items")
	mustNoErr(t, err)
	if len(got) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(got))
	}
	for index, want := range values {
		expectText(t, fmt.Sprintf("items[%d]", index), got[index], want)
	}
}

func TestMultibyteStringsStayIntact(t *testing.T) {
	cyrillic := strings.Repeat("ж", 5000)
	emoji := strings.Repeat("😀", 2500)

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddString("cyrillic", cyrillic))
	mustNoErr(t, c.AddString("emoji", emoji))

	restored := roundtripCollection(t, c)
	defer restored.Close()

	gotCyrillic, err := restored.GetString("cyrillic")
	mustNoErr(t, err)
	expectText(t, "cyrillic", gotCyrillic, cyrillic)
	if strings.ContainsRune(gotCyrillic, utf8.RuneError) {
		t.Errorf("cyrillic contains U+FFFD")
	}

	gotEmoji, err := restored.GetString("emoji")
	mustNoErr(t, err)
	expectText(t, "emoji", gotEmoji, emoji)
	if strings.ContainsRune(gotEmoji, utf8.RuneError) {
		t.Errorf("emoji contains U+FFFD")
	}
}

func TestEmbeddedNulInKeyAndValue(t *testing.T) {
	key := "k\x00ey"
	value := "a\x00b"

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddString(key, value))

	restored := roundtripCollection(t, c)
	defer restored.Close()

	keys, err := restored.Keys()
	mustNoErr(t, err)
	if len(keys) != 1 || keys[0] != key {
		t.Fatalf("Keys() = %q, want [%q]", keys, key)
	}
	got, err := restored.GetString(key)
	mustNoErr(t, err)
	expectText(t, "value", got, value)
}

func TestLongKeyIsEnumeratedAndReadBack(t *testing.T) {
	key := strings.Repeat("k", 600)

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt(key, 7))

	restored := roundtripCollection(t, c)
	defer restored.Close()

	keys, err := restored.Keys()
	mustNoErr(t, err)
	if len(keys) != 1 {
		t.Fatalf("Keys() returned %d keys, want 1", len(keys))
	}
	if keys[0] != key {
		t.Fatalf("Keys()[0] has %d bytes, want the 600-byte key", len(keys[0]))
	}
	got, err := restored.GetInt(key)
	expectValue(t, "long key value", got, err, int32(7))
}

func TestValueEndingWithBackslash(t *testing.T) {
	dir := `C:\dir\`

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddString("path", dir))
	mustNoErr(t, c.AddStringArray("paths", []string{`a\`, `b\\`, `\`}))

	nested := fon.NewCollection()
	mustNoErr(t, nested.AddString("path", `D:\x\`))
	mustNoErr(t, c.AddCollection("object", nested))

	item := fon.NewCollection()
	mustNoErr(t, item.AddString("path", `E:\y\`))
	mustNoErr(t, c.AddCollectionArray("objects", []*fon.Collection{item}))

	restored := roundtripCollection(t, c)
	defer restored.Close()

	gotPath, err := restored.GetString("path")
	mustNoErr(t, err)
	expectText(t, "path", gotPath, dir)

	gotPaths, err := restored.GetStringArray("paths")
	mustNoErr(t, err)
	wantPaths := []string{`a\`, `b\\`, `\`}
	if len(gotPaths) != len(wantPaths) {
		t.Fatalf("len(paths) = %d, want %d", len(gotPaths), len(wantPaths))
	}
	for index, want := range wantPaths {
		expectText(t, fmt.Sprintf("paths[%d]", index), gotPaths[index], want)
	}

	object, err := restored.GetCollection("object")
	mustNoErr(t, err)
	gotNested, err := object.GetString("path")
	mustNoErr(t, err)
	expectText(t, "object.path", gotNested, `D:\x\`)

	objects, err := restored.GetCollectionArray("objects")
	mustNoErr(t, err)
	gotItem, err := objects[0].GetString("path")
	mustNoErr(t, err)
	expectText(t, "objects[0].path", gotItem, `E:\y\`)
}

func TestControlCharactersRoundtrip(t *testing.T) {
	value := "\x01\x1b\x00tail\x1f"

	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddString("ctrl", value))
	mustNoErr(t, c.AddString("\x1bkey", "\x01"))
	mustNoErr(t, c.AddStringArray("ctrls", []string{"\x01", "\x1b"}))

	restored := roundtripCollection(t, c)
	defer restored.Close()

	got, err := restored.GetString("ctrl")
	mustNoErr(t, err)
	expectText(t, "ctrl", got, value)

	gotKey, err := restored.GetString("\x1bkey")
	mustNoErr(t, err)
	expectText(t, "\\x1bkey", gotKey, "\x01")

	gotArray, err := restored.GetStringArray("ctrls")
	mustNoErr(t, err)
	if len(gotArray) != 2 || gotArray[0] != "\x01" || gotArray[1] != "\x1b" {
		t.Errorf("ctrls = %q, want [\"\\x01\" \"\\x1b\"]", gotArray)
	}
}

func TestDelimitersInsideStrings(t *testing.T) {
	text := `a,b=c:d "quoted" [x] {y}`

	child := fon.NewCollection()
	mustNoErr(t, child.AddString("text", text))
	mustNoErr(t, child.AddStringArray("texts", []string{text, "],}"}))

	item := fon.NewCollection()
	mustNoErr(t, item.AddString("text", "}{,=:"))

	parent := fon.NewCollection()
	defer parent.Close()
	mustNoErr(t, parent.AddCollection("child", child))
	mustNoErr(t, parent.AddCollectionArray("items", []*fon.Collection{item}))

	restored := roundtripCollection(t, parent)
	defer restored.Close()

	nested, err := restored.GetCollection("child")
	mustNoErr(t, err)
	gotText, err := nested.GetString("text")
	mustNoErr(t, err)
	expectText(t, "child.text", gotText, text)

	gotTexts, err := nested.GetStringArray("texts")
	mustNoErr(t, err)
	if len(gotTexts) != 2 || gotTexts[0] != text || gotTexts[1] != "],}" {
		t.Errorf("child.texts = %q", gotTexts)
	}

	items, err := restored.GetCollectionArray("items")
	mustNoErr(t, err)
	gotItem, err := items[0].GetString("text")
	mustNoErr(t, err)
	expectText(t, "items[0].text", gotItem, "}{,=:")
}

// ---------------------------------------------------------------------------
// Design section 6.3: enumeration
// ---------------------------------------------------------------------------

func TestKeysOrderAndKinds(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt("b", 1))
	mustNoErr(t, c.AddString("a", "x"))
	child := fon.NewCollection()
	mustNoErr(t, child.AddBool("flag", true))
	mustNoErr(t, c.AddCollection("c", child))
	mustNoErr(t, c.AddDoubleArray("d", []float64{1}))

	want := []string{"b", "a", "c", "d"}
	wantKinds := []fon.Kind{
		{Type: fon.TypeInt},
		{Type: fon.TypeString},
		{Type: fon.TypeObject},
		{Type: fon.TypeDouble, Array: true},
	}

	check := func(label string, source *fon.Collection) {
		t.Helper()
		keys, err := source.Keys()
		mustNoErr(t, err)
		if fmt.Sprint(keys) != fmt.Sprint(want) {
			t.Fatalf("%s: Keys() = %v, want %v", label, keys, want)
		}
		for index, key := range keys {
			kind, ok := source.Kind(key)
			if !ok || kind != wantKinds[index] {
				t.Errorf("%s: Kind(%s) = %v, %v, want %v", label, key, kind, ok, wantKinds[index])
			}
		}
	}

	check("original", c)
	restored := roundtripCollection(t, c)
	defer restored.Close()
	check("restored", restored)
}

func TestDumpIdsAscendingAfterSparseChanges(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for _, id := range []uint64{10, 3, 7} {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("v", int32(id)))
		mustNoErr(t, d.Add(id, rec))
	}
	removed, err := d.Remove(3)
	mustNoErr(t, err)
	if !removed {
		t.Fatal("Remove(3) = false, want true")
	}
	rec := fon.NewCollection()
	mustNoErr(t, rec.AddInt("v", 1))
	mustNoErr(t, d.Add(1, rec))

	ids, err := d.Ids()
	mustNoErr(t, err)
	expectIDs(t, ids, []uint64{1, 7, 10})

	restored := roundtripDump(t, d)
	defer restored.Close()
	restoredIDs, err := restored.Ids()
	mustNoErr(t, err)
	expectIDs(t, restoredIDs, []uint64{0, 1, 2})
}

func TestEnumerateLargeDeserializedDumpQuickly(t *testing.T) {
	const records = 100000
	var builder strings.Builder
	for index := 0; index < records; index++ {
		builder.WriteString("v=i:")
		builder.WriteString(strconv.Itoa(index))
		builder.WriteByte('\n')
	}

	d, err := fon.DeserializeDumpFromBytes([]byte(builder.String()), 0)
	mustNoErr(t, err)
	defer d.Close()
	if d.Size() != records {
		t.Fatalf("Size() = %d, want %d", d.Size(), records)
	}

	start := time.Now()
	ids, err := d.Ids()
	mustNoErr(t, err)
	count := 0
	err = d.Range(func(id uint64, c *fon.Collection) bool {
		count++
		return true
	})
	mustNoErr(t, err)
	elapsed := time.Since(start)

	if len(ids) != records || count != records {
		t.Fatalf("enumerated %d ids and %d records, want %d", len(ids), count, records)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("enumerating %d records took %v, want well under 2s", records, elapsed)
	}
	t.Logf("enumerated %d records in %v", records, elapsed)
}

// ---------------------------------------------------------------------------
// Design section 6.4: mutation
// ---------------------------------------------------------------------------

func TestAddExistingKeyFails(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt("k", 1))

	expectErr(t, c.AddInt("k", 2), fon.ErrAlreadyExists)
	got, err := c.GetInt("k")
	expectValue(t, "k", got, err, int32(1))
}

func TestSetIsUpsert(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.SetString("k", "a"))
	mustNoErr(t, c.SetString("k", "b"))
	got, err := c.GetString("k")
	expectValue(t, "k", got, err, "b")

	mustNoErr(t, c.SetInt("k", 3))
	gotInt, err := c.GetInt("k")
	expectValue(t, "k as int", gotInt, err, int32(3))
	_, err = c.GetString("k")
	expectErr(t, err, fon.ErrTypeMismatch)
}

func TestDumpReplaceAbsentFails(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()

	rec := fon.NewCollection()
	mustNoErr(t, rec.AddInt("v", 1))
	expectErr(t, d.Replace(99, rec), fon.ErrNotFound)

	// The failed call leaves rec owned and usable.
	mustNoErr(t, rec.AddInt("w", 2))
	mustNoErr(t, d.Set(5, rec))
	_, err := rec.GetInt("w")
	expectErr(t, err, fon.ErrClosed)
}

func TestRemoveReportsPresence(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt("k", 1))

	removed, err := c.Remove("k")
	mustNoErr(t, err)
	if !removed {
		t.Fatal("Remove(k) = false, want true")
	}
	removed, err = c.Remove("k")
	mustNoErr(t, err)
	if removed {
		t.Fatal("second Remove(k) = true, want false")
	}
	if c.Has("k") {
		t.Fatal("Has(k) = true after Remove")
	}
}

func TestDumpRemoveManyCountsOnlyPresentIDs(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for _, id := range []uint64{1, 2, 3} {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("v", int32(id)))
		mustNoErr(t, d.Add(id, rec))
	}

	removed, err := d.RemoveMany([]uint64{1, 1, 2, 99})
	mustNoErr(t, err)
	if removed != 2 {
		t.Fatalf("RemoveMany = %d, want 2", removed)
	}
	ids, err := d.Ids()
	mustNoErr(t, err)
	expectIDs(t, ids, []uint64{3})
}

func TestRemoveKeepsOtherIDs(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for _, id := range []uint64{0, 1, 2} {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("v", int32(id*10)))
		mustNoErr(t, d.Add(id, rec))
	}
	removed, err := d.Remove(1)
	mustNoErr(t, err)
	if !removed {
		t.Fatal("Remove(1) = false, want true")
	}

	first, err := d.Get(0)
	mustNoErr(t, err)
	gotFirst, err := first.GetInt("v")
	expectValue(t, "record 0", gotFirst, err, int32(0))

	last, err := d.Get(2)
	mustNoErr(t, err)
	gotLast, err := last.GetInt("v")
	expectValue(t, "record 2", gotLast, err, int32(20))
}

func TestSerializeAfterRemoveRenumbersLines(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for _, id := range []uint64{0, 1, 2} {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("n", int32(id)))
		mustNoErr(t, d.Add(id, rec))
	}
	_, err := d.Remove(1)
	mustNoErr(t, err)

	data, err := d.SerializeToBytes(0)
	mustNoErr(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"n=i:0", "n=i:2"}
	if fmt.Sprint(lines) != fmt.Sprint(want) {
		t.Fatalf("serialized lines = %q, want %q", lines, want)
	}
}

// churnAllocations allocates and frees many native collections so that memory
// released earlier by the library is likely to be reused.
func churnAllocations(t *testing.T) {
	t.Helper()
	for index := 0; index < 20000; index++ {
		c := fon.NewCollection()
		mustNoErr(t, c.AddInt("v", int32(index)))
		c.Close()
	}
}

func TestAbsentRemoveKeepsRecordViewValid(t *testing.T) {
	d, err := fon.DeserializeDumpFromBytes([]byte("v=i:111\nv=i:222\nv=i:333\n"), 0)
	mustNoErr(t, err)
	defer d.Close()
	view, err := d.Get(1)
	mustNoErr(t, err)

	removed, err := d.Remove(999)
	mustNoErr(t, err)
	if removed {
		t.Fatal("Remove(999) = true, want false")
	}
	churnAllocations(t)

	got, err := view.GetInt("v")
	expectValue(t, "record 1 v", got, err, int32(222))
}

func TestAbsentRemoveManyKeepsRecordViewValid(t *testing.T) {
	d, err := fon.DeserializeDumpFromBytes([]byte("v=i:111\nv=i:222\nv=i:333\n"), 0)
	mustNoErr(t, err)
	defer d.Close()
	view, err := d.Get(2)
	mustNoErr(t, err)

	removed, err := d.RemoveMany([]uint64{500, 501})
	mustNoErr(t, err)
	if removed != 0 {
		t.Fatalf("RemoveMany(absent) = %d, want 0", removed)
	}
	churnAllocations(t)

	got, err := view.GetInt("v")
	expectValue(t, "record 2 v", got, err, int32(333))
}

// ---------------------------------------------------------------------------
// Design section 6.5: views
// ---------------------------------------------------------------------------

func TestRecordViewSurvivesRepeatedSet(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	rec := fon.NewCollection()
	mustNoErr(t, rec.AddInt("w", 1))
	mustNoErr(t, rec.AddString("name", "a"))
	mustNoErr(t, d.Add(0, rec))

	view, err := d.Get(0)
	mustNoErr(t, err)
	for step := 0; step < 3; step++ {
		w, err := view.GetInt("w")
		mustNoErr(t, err)
		mustNoErr(t, view.SetInt("w", w*2))
	}
	mustNoErr(t, view.SetString("name", "b"))

	w, err := view.GetInt("w")
	expectValue(t, "w", w, err, int32(8))
	name, err := view.GetString("name")
	expectValue(t, "name", name, err, "b")
}

func TestRecordViewStaleAfterRemoveOrReplace(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	first := fon.NewCollection()
	mustNoErr(t, first.AddInt("v", 1))
	second := fon.NewCollection()
	mustNoErr(t, second.AddInt("v", 2))
	mustNoErr(t, d.Add(0, first))
	mustNoErr(t, d.Add(1, second))

	removedView, err := d.Get(0)
	mustNoErr(t, err)
	replacedView, err := d.Get(1)
	mustNoErr(t, err)

	_, err = d.Remove(0)
	mustNoErr(t, err)
	_, err = removedView.GetInt("v")
	expectErr(t, err, fon.ErrStaleView)

	replacement := fon.NewCollection()
	mustNoErr(t, replacement.AddInt("v", 3))
	mustNoErr(t, d.Set(1, replacement))
	_, err = replacedView.GetInt("v")
	expectErr(t, err, fon.ErrStaleView)
}

func TestRecordViewOfOtherIDUnaffected(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for _, id := range []uint64{0, 1} {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("v", int32(id)))
		mustNoErr(t, d.Add(id, rec))
	}
	view, err := d.Get(0)
	mustNoErr(t, err)

	replacement := fon.NewCollection()
	mustNoErr(t, replacement.AddInt("v", 9))
	mustNoErr(t, d.Set(1, replacement))
	got, err := view.GetInt("v")
	expectValue(t, "after replace of 1", got, err, int32(0))

	_, err = d.Remove(1)
	mustNoErr(t, err)
	got, err = view.GetInt("v")
	expectValue(t, "after remove of 1", got, err, int32(0))
}

func TestNestedViewDanglingChildIsStale(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	root := fon.NewCollection()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("child", child))
	mustNoErr(t, d.Add(0, root))

	rec, err := d.Get(0)
	mustNoErr(t, err)
	nested, err := rec.GetCollection("child")
	mustNoErr(t, err)
	mustNoErr(t, rec.SetInt("child", 5))

	_, err = nested.GetInt("x")
	expectErr(t, err, fon.ErrStaleView)
}

func TestNestedViewOfOwnedRootDanglingChildIsStale(t *testing.T) {
	root := fon.NewCollection()
	defer root.Close()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("child", child))

	nested, err := root.GetCollection("child")
	mustNoErr(t, err)
	mustNoErr(t, root.SetInt("child", 5))

	_, err = nested.GetInt("x")
	expectErr(t, err, fon.ErrStaleView)
}

func TestNestedViewMutationKeepsItValidAndStalesSiblings(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	rec := fon.NewCollection()
	first := fon.NewCollection()
	mustNoErr(t, first.AddInt("x", 1))
	second := fon.NewCollection()
	mustNoErr(t, second.AddInt("y", 2))
	mustNoErr(t, rec.AddCollection("a", first))
	mustNoErr(t, rec.AddCollection("b", second))
	mustNoErr(t, d.Add(0, rec))

	view, err := d.Get(0)
	mustNoErr(t, err)
	nestedA, err := view.GetCollection("a")
	mustNoErr(t, err)
	nestedB, err := view.GetCollection("b")
	mustNoErr(t, err)

	mustNoErr(t, nestedA.SetInt("x", 10))
	gotA, err := nestedA.GetInt("x")
	expectValue(t, "a.x", gotA, err, int32(10))

	_, err = nestedB.GetInt("y")
	expectErr(t, err, fon.ErrStaleView)

	// The record view itself stays valid after a change in its tree.
	fresh, err := view.GetCollection("b")
	mustNoErr(t, err)
	gotB, err := fresh.GetInt("y")
	expectValue(t, "fresh b.y", gotB, err, int32(2))
}

func TestNestedViewOfOwnedCollectionStaleAfterOwnerMutation(t *testing.T) {
	root := fon.NewCollection()
	defer root.Close()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("child", child))

	nested, err := root.GetCollection("child")
	mustNoErr(t, err)
	mustNoErr(t, nested.SetInt("x", 9))
	gotX, err := nested.GetInt("x")
	expectValue(t, "nested x", gotX, err, int32(9))

	mustNoErr(t, root.SetInt("other", 1))
	_, err = nested.GetInt("x")
	expectErr(t, err, fon.ErrStaleView)
}

func TestViewsReportClosedAfterOwnerDisposed(t *testing.T) {
	d := fon.NewDump()
	root := fon.NewCollection()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("child", child))
	mustNoErr(t, d.Add(0, root))
	rec, err := d.Get(0)
	mustNoErr(t, err)
	d.Close()
	_, err = rec.GetInt("child")
	expectErr(t, err, fon.ErrClosed)

	owned := fon.NewCollection()
	ownedChild := fon.NewCollection()
	mustNoErr(t, ownedChild.AddInt("x", 1))
	mustNoErr(t, owned.AddCollection("child", ownedChild))
	nested, err := owned.GetCollection("child")
	mustNoErr(t, err)
	owned.Close()
	_, err = nested.GetInt("x")
	expectErr(t, err, fon.ErrClosed)
}

// collectedProbe returns a channel that is closed once a probe object, created here
// and referenced by nothing else, has been finalized. Create it in the same scope
// as the owner it tracks, so that both become unreachable together.
func collectedProbe() <-chan struct{} {
	done := make(chan struct{})
	probe := new([8]byte)
	runtime.SetFinalizer(probe, func(*[8]byte) {
		close(done)
	})
	return done
}

// waitCollected runs the collector until every probe has been finalized. Because
// finalizers run asynchronously, the owners dropped with the probes are only known
// to be gone once their probes have run.
func waitCollected(t *testing.T, probes ...<-chan struct{}) {
	t.Helper()
	for _, done := range probes {
		collected := false
		for attempt := 0; attempt < 500 && !collected; attempt++ {
			runtime.GC()
			select {
			case <-done:
				collected = true
			case <-time.After(10 * time.Millisecond):
			}
		}
		if !collected {
			t.Fatal("owner was not finalized within 5s")
		}
	}
}

// recordViewWithoutDump returns a record view whose dump is unreachable from the
// caller: the view alone must keep the dump alive. The probe reports when the dump
// has been collected.
func recordViewWithoutDump(t *testing.T) (*fon.Collection, <-chan struct{}) {
	t.Helper()
	d := fon.NewDump()
	rec := fon.NewCollection()
	mustNoErr(t, rec.AddInt("v", 7))
	mustNoErr(t, d.Add(0, rec))
	view, err := d.Get(0)
	mustNoErr(t, err)
	return view, collectedProbe()
}

// nestedViewWithoutOwners returns a nested view whose dump and root are
// unreachable from the caller. The probe reports when they have been collected.
func nestedViewWithoutOwners(t *testing.T) (*fon.Collection, <-chan struct{}) {
	t.Helper()
	d := fon.NewDump()
	root := fon.NewCollection()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 42))
	mustNoErr(t, root.AddCollection("child", child))
	mustNoErr(t, d.Add(0, root))
	rec, err := d.Get(0)
	mustNoErr(t, err)
	nested, err := rec.GetCollection("child")
	mustNoErr(t, err)
	return nested, collectedProbe()
}

// ownedNestedViewWithoutRoot returns a nested view whose owned root is unreachable
// from the caller. The probe reports when the root has been collected.
func ownedNestedViewWithoutRoot(t *testing.T) (*fon.Collection, <-chan struct{}) {
	t.Helper()
	root := fon.NewCollection()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 43))
	mustNoErr(t, root.AddCollection("child", child))
	nested, err := root.GetCollection("child")
	mustNoErr(t, err)
	return nested, collectedProbe()
}

func TestViewKeepsOwnerAlive(t *testing.T) {
	record, recordDone := recordViewWithoutDump(t)
	nested, nestedDone := nestedViewWithoutOwners(t)
	owned, ownedDone := ownedNestedViewWithoutRoot(t)

	waitCollected(t, recordDone, nestedDone, ownedDone)

	gotRecord, err := record.GetInt("v")
	expectValue(t, "record view v", gotRecord, err, int32(7))
	gotNested, err := nested.GetInt("x")
	expectValue(t, "nested view x", gotNested, err, int32(42))
	gotOwned, err := owned.GetInt("x")
	expectValue(t, "owned nested view x", gotOwned, err, int32(43))
}

// ---------------------------------------------------------------------------
// Design section 6.6: ownership
// ---------------------------------------------------------------------------

func TestViewCannotBeInsertedButCloneCan(t *testing.T) {
	root := fon.NewCollection()
	defer root.Close()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("child", child))
	nested, err := root.GetCollection("child")
	mustNoErr(t, err)

	parent := fon.NewCollection()
	defer parent.Close()
	expectCode(t, parent.AddCollection("view", nested), fon.ResultInvalidArgument)

	dump := fon.NewDump()
	defer dump.Close()
	expectCode(t, dump.Add(0, nested), fon.ResultInvalidArgument)

	clone, err := nested.Clone()
	mustNoErr(t, err)
	mustNoErr(t, parent.AddCollection("view", clone))

	copied, err := parent.GetCollection("view")
	mustNoErr(t, err)
	got, err := copied.GetInt("x")
	expectValue(t, "cloned x", got, err, int32(1))
}

func TestTransferredWrapperIsUnusable(t *testing.T) {
	parent := fon.NewCollection()
	defer parent.Close()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))
	mustNoErr(t, parent.AddCollection("c", child))

	_, err := child.GetInt("x")
	expectErr(t, err, fon.ErrClosed)
	expectErr(t, parent.AddCollection("again", child), fon.ErrClosed)

	dump := fon.NewDump()
	defer dump.Close()
	other := fon.NewCollection()
	mustNoErr(t, other.AddInt("x", 1))
	mustNoErr(t, dump.Add(0, other))
	_, err = other.GetInt("x")
	expectErr(t, err, fon.ErrClosed)
}

func TestFailedPutKeepsChildOwned(t *testing.T) {
	parent := fon.NewCollection()
	defer parent.Close()
	mustNoErr(t, parent.AddInt("taken", 1))

	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 5))
	expectErr(t, parent.AddCollection("taken", child), fon.ErrAlreadyExists)

	got, err := child.GetInt("x")
	expectValue(t, "child x", got, err, int32(5))
	mustNoErr(t, parent.SetCollection("taken", child))
	_, err = child.GetInt("x")
	expectErr(t, err, fon.ErrClosed)
}

func TestRootCannotBeInsertedIntoItsOwnNestedView(t *testing.T) {
	root := fon.NewCollection()
	defer root.Close()
	inner := fon.NewCollection()
	mustNoErr(t, inner.AddInt("x", 1))
	mustNoErr(t, root.AddCollection("inner", inner))

	nested, err := root.GetCollection("inner")
	mustNoErr(t, err)
	expectCode(t, nested.AddCollection("loop", root), fon.ResultInvalidArgument)
	expectCode(t, root.AddCollection("self", root), fon.ResultInvalidArgument)

	// The failed inserts leave root owned and usable.
	if !root.Has("inner") {
		t.Fatal("root lost its inner collection after a rejected insert")
	}
	got, err := nested.GetInt("x")
	expectValue(t, "nested x", got, err, int32(1))
}

func TestSameChildTwiceInArrayIsRejected(t *testing.T) {
	parent := fon.NewCollection()
	defer parent.Close()
	child := fon.NewCollection()
	mustNoErr(t, child.AddInt("x", 1))

	expectCode(t, parent.AddCollectionArray("arr", []*fon.Collection{child, child}), fon.ResultInvalidArgument)
	got, err := child.GetInt("x")
	expectValue(t, "child x", got, err, int32(1))
	if parent.Has("arr") {
		t.Fatal("parent received the array after a rejected put")
	}
}

func TestCloneIsDeep(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt("x", 1))

	clone, err := c.Clone()
	mustNoErr(t, err)
	defer clone.Close()
	mustNoErr(t, clone.SetInt("x", 9))

	original, err := c.GetInt("x")
	expectValue(t, "original x", original, err, int32(1))
	copied, err := clone.GetInt("x")
	expectValue(t, "clone x", copied, err, int32(9))
}

// ---------------------------------------------------------------------------
// Design section 6.7: thread settings
// ---------------------------------------------------------------------------

func TestMaxThreadsSerializeIdenticalToDefault(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	for id := 0; id < 1000; id++ {
		rec := fon.NewCollection()
		mustNoErr(t, rec.AddInt("v", int32(id)))
		mustNoErr(t, rec.AddString("s", "row "+strconv.Itoa(id)))
		mustNoErr(t, d.Add(uint64(id), rec))
	}

	baseline, err := d.SerializeToBytes(0)
	mustNoErr(t, err)
	for _, threads := range []int{1, 2} {
		data, err := d.SerializeToBytes(threads)
		mustNoErr(t, err)
		if string(data) != string(baseline) {
			t.Errorf("SerializeToBytes(%d) differs from SerializeToBytes(0)", threads)
		}

		restored, err := fon.DeserializeDumpFromBytes(baseline, threads)
		mustNoErr(t, err)
		again, err := restored.SerializeToBytes(0)
		mustNoErr(t, err)
		restored.Close()
		if string(again) != string(baseline) {
			t.Errorf("deserialize(%d) then serialize differs from the baseline", threads)
		}
	}

	path := filepath.Join(t.TempDir(), "threads.fon")
	mustNoErr(t, d.SerializeToFile(path, 2))
	fromFile, err := fon.DeserializeDumpFromFile(path, 1)
	mustNoErr(t, err)
	defer fromFile.Close()
	if fromFile.Size() != 1000 {
		t.Fatalf("Size() after file round-trip = %d, want 1000", fromFile.Size())
	}
}

// ---------------------------------------------------------------------------
// Design section 6.8: file errors and error mapping
// ---------------------------------------------------------------------------

func TestDeserializeMissingFileReportsFileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.fon")
	_, err := fon.DeserializeDumpFromFile(path, 0)
	expectCode(t, err, fon.ResultFileNotFound)
}

func TestErrorMappingToSentinels(t *testing.T) {
	c := fon.NewCollection()
	defer c.Close()
	mustNoErr(t, c.AddInt("n", 1))

	_, err := c.GetInt("missing")
	expectErr(t, err, fon.ErrNotFound)
	_, err = c.GetString("n")
	expectErr(t, err, fon.ErrTypeMismatch)
	if c.Has("missing") {
		t.Fatal("Has(missing) = true")
	}
	if _, ok := c.Kind("missing"); ok {
		t.Fatal("Kind(missing) reported a kind")
	}

	d := fon.NewDump()
	defer d.Close()
	_, err = d.Get(42)
	expectErr(t, err, fon.ErrNotFound)
}

// TestReservedKeyCharactersAreRejected covers D15: keys containing a character
// that the text format reserves are refused with an argument error, and a child
// passed to such a rejected put stays owned and usable.
func TestReservedKeyCharactersAreRejected(t *testing.T) {
	keys := []string{"a=b", `a"b`, "a{b", "a}b", "a[b", "a]b", "a\rb", "a\nb"}
	for _, key := range keys {
		parent := fon.NewCollection()
		child := fon.NewCollection()
		mustNoErr(t, child.AddInt("x", 5))

		expectCode(t, parent.AddInt(key, 1), fon.ResultInvalidArgument)
		expectCode(t, parent.AddCollection(key, child), fon.ResultInvalidArgument)
		if parent.Has(key) {
			t.Fatalf("rejected key %q was stored", key)
		}

		got, err := child.GetInt("x")
		expectValue(t, "child x after rejected key", got, err, int32(5))

		mustNoErr(t, parent.AddCollection("ok", child))
		copied, err := parent.GetCollection("ok")
		mustNoErr(t, err)
		gotCopy, err := copied.GetInt("x")
		expectValue(t, "transferred child x", gotCopy, err, int32(5))

		parent.Close()
		child.Close()
	}
}
