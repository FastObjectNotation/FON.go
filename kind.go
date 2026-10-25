package fon

// Type names the element type of a stored value, as reported by Collection.Kind.
type Type string

// Value type names.
const (
	TypeByte   Type = "byte"
	TypeShort  Type = "short"
	TypeInt    Type = "int"
	TypeUInt   Type = "uint"
	TypeLong   Type = "long"
	TypeULong  Type = "ulong"
	TypeFloat  Type = "float"
	TypeDouble Type = "double"
	TypeBool   Type = "bool"
	TypeString Type = "string"
	TypeRaw    Type = "raw"
	TypeObject Type = "object"
)

// kindArrayFlag is FON_KIND_ARRAY from fon.h: set in a native kind value when the
// value is an array.
const kindArrayFlag int32 = 0x100

// Kind describes a stored value: its element type and whether it is an array.
type Kind struct {
	Type  Type
	Array bool
}

// kindFromCode decodes a native kind value (type code, plus kindArrayFlag for
// arrays). It reports false for codes it does not know.
func kindFromCode(code int32) (Kind, bool) {
	isArray := code&kindArrayFlag != 0
	var name Type
	switch code & 0xff {
	case 'e':
		name = TypeByte
	case 't':
		name = TypeShort
	case 'i':
		name = TypeInt
	case 'u':
		name = TypeUInt
	case 'l':
		name = TypeLong
	case 'g':
		name = TypeULong
	case 'f':
		name = TypeFloat
	case 'd':
		name = TypeDouble
	case 'b':
		name = TypeBool
	case 's':
		name = TypeString
	case 'r':
		name = TypeRaw
	case 'o':
		name = TypeObject
	default:
		return Kind{}, false
	}
	return Kind{Type: name, Array: isArray}, true
}
