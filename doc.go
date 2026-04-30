// Package clone provides Clone and CloneSlowly: stdlib-only, reflect-based
// deep clone for Go values, with an optional Cloneable[T] escape hatch for
// types that want to provide their own fast path.
//
// # Contract
//
//   - Exported pointer / slice / map / struct / interface / array fields are
//     deep-cloned recursively.
//   - Scalar exported fields and ALL unexported fields are shallow-copied via
//     a whole-struct reflect.Value.Set. This means time.Time and similar
//     scalar-like structs round-trip correctly (wall + monotonic clock +
//     location pointer all preserved), but structs whose UNEXPORTED fields
//     hold pointer-like data will share those values with the original.
//   - Clone does NOT support pointer cycles; calling it on a cyclic graph
//     will infinite-loop. Use CloneSlowly for cycles.
//   - sync.Mutex / sync.WaitGroup and similar "no-copy" types are not given
//     special handling: their unexported state would be shallow-shared. Do
//     not clone structs containing live mutexes.
//
// # Performance
//
// Stdlib-only reflect; no unsafe, no third-party dependencies on the
// production side. Roughly competitive with reflect+unsafe libraries like
// huandu/go-clone for typical workloads — faster on simple structs and on
// the cycle path, similar on slices and trees, ~1.8× slower on structs
// with several pointer-typed exported fields. See benchmarks/ for numbers.
//
// # Escape hatch
//
// For hot paths where reflect overhead is unacceptable, types may implement
// Cloneable[T]:
//
//	type Foo struct{ Bar string }
//	func (f *Foo) Clone() *Foo { c := *f; return &c }
//
// Clone[Foo] will invoke f.Clone() and skip the reflect path entirely.
// CloneSlowly does NOT consult Cloneable[T] — it always uses reflect with
// cycle detection — so a Foo passed to CloneSlowly takes the reflect path.
package clone
