package clone

import (
	"reflect"
)

// Cloneable defines the interface for types that provide their own deep clone.
//
// When Clone is called on a value whose type satisfies Cloneable[T], the type's
// own Clone method is invoked instead of the default reflect-based deep copy.
// CloneSlowly does NOT consult this interface: it always uses the reflect path
// (with cycle detection). This preserves the historical behavior split between
// Clone (fast path with escape hatch) and CloneSlowly (cycle-aware deep copy).
type Cloneable[T any] interface {
	// Clone creates a deep copy of the object.
	Clone() *T
}

// Clone creates a deep copy of o.
//
// Strategy: stdlib reflect-only deep walk (no unsafe, no third-party).
// Exported pointer / slice / map / struct / interface / array fields are
// recursively cloned. Scalar fields and ALL unexported fields are
// shallow-copied via a whole-struct reflect.Value.Set, which carries
// unexported data along without bypassing reflect's exported-flag check.
//
// Implications:
//   - time.Time and similar scalar-like structs round-trip correctly: their
//     unexported wall/ext/loc fields are preserved by the parent struct's
//     whole-value Set.
//   - Structs whose UNEXPORTED fields hold pointer / slice / map data will
//     have those values shallow-shared with the original, NOT deep-cloned.
//     Mutations to such shared values would be visible to both. Callers
//     who require full deep semantics for unexported fields must keep the
//     "all fields must be exported" contract documented here.
//
// Pointer cycles are NOT supported by Clone: calling Clone on a cyclic graph
// will infinite-loop. Use CloneSlowly for cycles.
//
// If o == nil, Clone returns nil (typed nil *T).
//
// If *T satisfies Cloneable[T], the type's Clone method is called and the
// reflect path is skipped. Implement Cloneable[T] when you need a custom
// fast path, want to avoid reflect for performance, or need behavior that
// the reflect walker cannot express.
func Clone[T any](o *T) *T {
	if o == nil {
		return nil
	}
	if c, ok := (interface{})(o).(Cloneable[T]); ok {
		return c.Clone()
	}
	out := new(T)
	deepCopy(reflect.ValueOf(out).Elem(), reflect.ValueOf(o).Elem(), nil)
	return out
}

// CloneSlowly creates a deep copy of o, supporting pointer cycles via a
// visited-pointer map.
//
// Use this function when the object graph may contain self-references or
// mutual cycles between pointers (e.g. a doubly-linked list, parent-back
// pointers, mutually-referencing structs). The visited map is allocated
// per call, so concurrent callers do not share state.
//
// Unlike Clone, CloneSlowly does NOT consult Cloneable[T]: the reflect path
// always runs. This preserves the historical behavior split between the two
// functions.
//
// If o == nil, CloneSlowly returns nil (typed nil *T).
func CloneSlowly[T any](o *T) *T {
	if o == nil {
		return nil
	}
	out := new(T)
	// Pre-register the input pointer so that any cycle that references back
	// to o resolves to out instead of triggering a fresh allocation. Without
	// this, a self-cycle (n.Self = n) would clone n into out, then encounter
	// the Self field, fail to find o in visited, and allocate a SECOND node
	// — producing c.Self != c.
	visited := map[uintptr]reflect.Value{
		reflect.ValueOf(o).Pointer(): reflect.ValueOf(out),
	}
	deepCopy(reflect.ValueOf(out).Elem(), reflect.ValueOf(o).Elem(), visited)
	return out
}

// deepCopy writes a deep copy of src into dst.
//
// visited is non-nil for CloneSlowly (cycle detection) and nil for Clone (no
// cycle support, slightly faster). When non-nil, visited maps source pointer
// addresses to the corresponding allocated destination reflect.Value, so that
// repeated visits to the same source pointer produce a single shared
// destination pointer — preserving cyclic structure with independent identity.
//
// Pre-condition: dst must be settable (CanSet() == true) and have the same
// type as src. Callers from Clone / CloneSlowly satisfy this by allocating
// dst via new(T) and unwrapping with reflect.ValueOf(out).Elem().
func deepCopy(dst, src reflect.Value, visited map[uintptr]reflect.Value) {
	switch src.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String,
		reflect.Func, reflect.Chan, reflect.UnsafePointer:
		// Scalar / pointer-like-but-opaque kinds: shallow copy by value.
		// Func / Chan / UnsafePointer are shallow-copied just like huandu;
		// they are outside the documented "JSON-serializable" contract but
		// preserving the reference value is the closest sensible default.
		dst.Set(src)

	case reflect.Ptr:
		if src.IsNil() {
			return
		}
		if visited != nil {
			if existing, ok := visited[src.Pointer()]; ok {
				dst.Set(existing)
				return
			}
		}
		elem := reflect.New(src.Type().Elem())
		// Register the new allocation BEFORE recursing so that a cycle
		// through src.Elem() resolves to this elem rather than triggering
		// infinite recursion.
		if visited != nil {
			visited[src.Pointer()] = elem
		}
		deepCopy(elem.Elem(), src.Elem(), visited)
		dst.Set(elem)

	case reflect.Slice:
		if src.IsNil() {
			return
		}
		n := src.Len()
		out := reflect.MakeSlice(src.Type(), n, n)
		for i := 0; i < n; i++ {
			deepCopy(out.Index(i), src.Index(i), visited)
		}
		dst.Set(out)

	case reflect.Map:
		if src.IsNil() {
			return
		}
		out := reflect.MakeMapWithSize(src.Type(), src.Len())
		keyType := src.Type().Key()
		elemType := src.Type().Elem()
		for iter := src.MapRange(); iter.Next(); {
			k := reflect.New(keyType).Elem()
			v := reflect.New(elemType).Elem()
			deepCopy(k, iter.Key(), visited)
			deepCopy(v, iter.Value(), visited)
			out.SetMapIndex(k, v)
		}
		dst.Set(out)

	case reflect.Struct:
		// Whole-struct shallow copy first: this carries ALL fields including
		// unexported ones. reflect's exported-flag check fires on per-field
		// Field(i).Set(), NOT on whole-struct Set on a settable destination,
		// so this works without unsafe. After the shallow copy, we walk
		// exported fields and re-clone any that hold pointer-like data.
		dst.Set(src)
		t := src.Type()
		for i := 0; i < src.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			switch src.Field(i).Kind() {
			case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface, reflect.Struct, reflect.Array:
				deepCopy(dst.Field(i), src.Field(i), visited)
			}
		}

	case reflect.Array:
		// Arrays are value types; the parent context already allocated dst
		// of the right size. Recurse per element so non-scalar elements get
		// deep-cloned.
		for i := 0; i < src.Len(); i++ {
			deepCopy(dst.Index(i), src.Index(i), visited)
		}

	case reflect.Interface:
		if src.IsNil() {
			return
		}
		elem := src.Elem()
		c := reflect.New(elem.Type()).Elem()
		deepCopy(c, elem, visited)
		dst.Set(c)

	default:
		panic("clone: unsupported reflect.Kind " + src.Kind().String() + " for type " + src.Type().String())
	}
}
