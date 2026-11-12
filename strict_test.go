package fon_test

import (
	"math"
	"strings"
	"testing"

	fon "github.com/FastObjectNotation/FON.go"
)

// TestMalformedTextIsRejected checks that text the parser has to guess about is
// a parse error, including the bool array that used to hang the parser.
func TestMalformedTextIsRejected(t *testing.T) {
	lines := []string{
		"a=b:[[]]",
		"a=i:12abc",
		"a=i:+",
		"a=b:false",
		"a=i:1,junk",
		"a=i:1,a=i:2",
		"a=i:1,",
		"a=i:[1,,2]",
		`a=s:"\q"`,
	}
	for _, line := range lines {
		c, err := fon.DeserializeCollectionFromBytes([]byte(line))
		if c != nil {
			c.Close()
			t.Fatalf("%q was accepted", line)
		}
		expectCode(t, err, fon.ResultParseFailed)
	}
}

// TestDumpParseErrorNamesTheLine checks that a parse error in a dump starts with
// the 1-based line number, for every thread count.
func TestDumpParseErrorNamesTheLine(t *testing.T) {
	for _, threads := range []int{1, 2, 0} {
		d, err := fon.DeserializeDumpFromBytes([]byte("a=i:1\na=b:[[]]\nc=i:3\n"), threads)
		if d != nil {
			d.Close()
			t.Fatalf("malformed dump was accepted with %d threads", threads)
		}
		expectCode(t, err, fon.ResultParseFailed)
		if !strings.HasPrefix(err.Error(), "line 2: ") {
			t.Fatalf("error = %q, want it to start with \"line 2: \"", err.Error())
		}
	}
}

// TestEmptyRecordSurvivesSaveAndLoad checks that an empty record is written as
// an empty line and read back as a record, so ids keep matching line numbers.
func TestEmptyRecordSurvivesSaveAndLoad(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	first := fon.NewCollection()
	mustNoErr(t, first.AddInt("x", 1))
	mustNoErr(t, d.Add(0, first))
	mustNoErr(t, d.Add(1, fon.NewCollection()))
	third := fon.NewCollection()
	mustNoErr(t, third.AddInt("x", 3))
	mustNoErr(t, d.Add(2, third))

	data, err := d.SerializeToBytes(0)
	mustNoErr(t, err)
	expectText(t, "dump text", string(data), "x=i:1\n\nx=i:3\n")

	loaded, err := fon.DeserializeDumpFromBytes(data, 0)
	mustNoErr(t, err)
	defer loaded.Close()
	if loaded.Size() != 3 {
		t.Fatalf("loaded %d records, want 3", loaded.Size())
	}
	empty, err := loaded.Get(1)
	mustNoErr(t, err)
	if empty.Size() != 0 {
		t.Fatalf("record 1 has %d fields, want 0", empty.Size())
	}
	last, err := loaded.Get(2)
	mustNoErr(t, err)
	got, err := last.GetInt("x")
	expectValue(t, "x of record 2", got, err, int32(3))
}

// TestThreadCountsAndDepthOutsideInt32AreSaturated checks that an int outside
// the 32-bit range is clamped instead of wrapping around to another value.
func TestThreadCountsAndDepthOutsideInt32AreSaturated(t *testing.T) {
	d := fon.NewDump()
	defer d.Close()
	c := fon.NewCollection()
	mustNoErr(t, c.AddInt("n", 1))
	mustNoErr(t, d.Add(0, c))

	huge := int(int64(math.MaxInt32) + 1)
	for _, threads := range []int{math.MaxInt, math.MinInt, huge} {
		data, err := d.SerializeToBytes(threads)
		mustNoErr(t, err)
		loaded, err := fon.DeserializeDumpFromBytes(data, threads)
		mustNoErr(t, err)
		if loaded.Size() != 1 {
			t.Fatalf("loaded %d records with maxThreads %d, want 1", loaded.Size(), threads)
		}
		loaded.Close()
	}

	// 2^32 + 1 used to wrap to 1, which rejects every nested object.
	fon.SetMaxDepth(int(int64(1)<<32 + 1))
	defer fon.SetMaxDepth(64)
	nested, err := fon.DeserializeCollectionFromBytes([]byte("a=o:{b=o:{c=i:1}}"))
	mustNoErr(t, err)
	nested.Close()
}

// TestCollectionCannotBeStoredInsideItsOwnNestedCollection checks that a put
// which would make a tree contain itself is refused and leaves the tree intact.
func TestCollectionCannotBeStoredInsideItsOwnNestedCollection(t *testing.T) {
	root := fon.NewCollection()
	defer root.Close()
	middle := fon.NewCollection()
	mustNoErr(t, middle.AddCollection("leaf", fon.NewCollection()))
	mustNoErr(t, root.AddCollection("middle", middle))

	middleView, err := root.GetCollection("middle")
	mustNoErr(t, err)
	leafView, err := middleView.GetCollection("leaf")
	mustNoErr(t, err)

	expectCode(t, leafView.AddCollection("back", root), fon.ResultInvalidArgument)
	expectCode(t, leafView.AddCollectionArray("backs", []*fon.Collection{root}), fon.ResultInvalidArgument)

	text, err := root.SerializeToString()
	mustNoErr(t, err)
	expectText(t, "tree after the refused puts", text, "middle=o:{leaf=o:{}}")
}
