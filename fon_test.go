package fon_test

import (
	"strings"
	"testing"

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

	// fon_dump_add takes ownership of c; we must not Close c after this call.
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
	// col is borrowed; do NOT close it.

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
// collections (fon_collection_add_collection_array /
// fon_collection_get_collection_array).
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
	// items are borrowed — do NOT close them.

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
