package grpcrunner

import (
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Template renders an example JSON body for md with every field present,
// recursing into nested messages (cycles are cut off as {}).
func Template(md protoreflect.MessageDescriptor) string {
	var b strings.Builder
	writeMessage(&b, md, 0, map[protoreflect.FullName]bool{})
	return b.String()
}

func indent(b *strings.Builder, n int) {
	b.WriteString(strings.Repeat("  ", n))
}

func writeMessage(b *strings.Builder, md protoreflect.MessageDescriptor, depth int, seen map[protoreflect.FullName]bool) {
	if wk, ok := wellKnown(md); ok {
		b.WriteString(wk)
		return
	}
	fields := md.Fields()
	if fields.Len() == 0 || seen[md.FullName()] || depth > 8 {
		b.WriteString("{}")
		return
	}
	seen[md.FullName()] = true
	defer delete(seen, md.FullName())
	b.WriteString("{\n")
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		indent(b, depth+1)
		b.WriteString(strconv.Quote(f.JSONName()))
		b.WriteString(": ")
		switch {
		case f.IsMap():
			b.WriteString("{")
			b.WriteString(strconv.Quote(keyExample(f.MapKey())))
			b.WriteString(": ")
			writeValue(b, f.MapValue(), depth+1, seen)
			b.WriteString("}")
		case f.IsList():
			b.WriteString("[")
			writeValue(b, f, depth+1, seen)
			b.WriteString("]")
		default:
			writeValue(b, f, depth+1, seen)
		}
		if i < fields.Len()-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	indent(b, depth)
	b.WriteString("}")
}

func writeValue(b *strings.Builder, f protoreflect.FieldDescriptor, depth int, seen map[protoreflect.FullName]bool) {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		writeMessage(b, f.Message(), depth, seen)
	case protoreflect.EnumKind:
		vals := f.Enum().Values()
		if vals.Len() > 0 {
			b.WriteString(strconv.Quote(string(vals.Get(0).Name())))
		} else {
			b.WriteString(`""`)
		}
	case protoreflect.BoolKind:
		b.WriteString("false")
	case protoreflect.StringKind:
		b.WriteString(`""`)
	case protoreflect.BytesKind:
		b.WriteString(`""`)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		b.WriteString(`"0"`) // protojson encodes 64-bit ints as strings
	default:
		b.WriteString("0")
	}
}

func keyExample(f protoreflect.FieldDescriptor) string {
	if f.Kind() == protoreflect.StringKind {
		return "key"
	}
	if f.Kind() == protoreflect.BoolKind {
		return "true"
	}
	return "0"
}

func wellKnown(md protoreflect.MessageDescriptor) (string, bool) {
	switch md.FullName() {
	case "google.protobuf.Timestamp":
		return `"1970-01-01T00:00:00Z"`, true
	case "google.protobuf.Duration":
		return `"0s"`, true
	case "google.protobuf.Struct":
		return "{}", true
	case "google.protobuf.Value":
		return "null", true
	case "google.protobuf.ListValue":
		return "[]", true
	case "google.protobuf.Empty":
		return "{}", true
	case "google.protobuf.Any":
		return `{"@type": ""}`, true
	case "google.protobuf.FieldMask":
		return `""`, true
	case "google.protobuf.StringValue", "google.protobuf.BytesValue",
		"google.protobuf.Int64Value", "google.protobuf.UInt64Value":
		return `""`, true
	case "google.protobuf.BoolValue":
		return "false", true
	case "google.protobuf.Int32Value", "google.protobuf.UInt32Value",
		"google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return "0", true
	}
	return "", false
}
