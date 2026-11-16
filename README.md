# clone-go

[![Go Reference](https://pkg.go.dev/badge/github.com/pixie-sh/clone-go.svg)](https://pkg.go.dev/github.com/pixie-sh/clone-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Made by [Pixie](https://pixie.sh).

Stdlib-only, reflect-based deep clone for Go, with an optional `Cloneable[T]` escape hatch.

```go
import "github.com/pixie-sh/clone-go"
```

## Why

This module exists because `github.com/huandu/go-clone/generic` repeatedly blocked downstream consumers of [`pixie-sh/core-go`][core-go] with `go.sum` checksum-mismatch failures. We needed deep clone to keep working without depending on a module whose download path was unreliable.

`clone-go` ships zero non-stdlib production dependencies (its only direct dep is `reflect`), so anyone fetching it gets exactly the implementation, nothing else. The benchmark harness lives in a [sub-module](./benchmarks/) so even huandu's code stays out of consumers' graphs.

[core-go]: https://github.com/pixie-sh/core-go

## Install

```sh
go get github.com/pixie-sh/clone-go
```

Requires Go 1.24.4 or newer (uses generics).

## Usage

### Basic clone

```go
import clone "github.com/pixie-sh/clone-go"

type User struct {
    Name string
    Tags []string
}

orig := &User{Name: "Alice", Tags: []string{"admin"}}
c := clone.Clone(orig)

c.Name = "Bob"
c.Tags[0] = "guest"

// orig.Name is still "Alice" and orig.Tags[0] is still "admin"
```

### Cloneable escape hatch (skip reflect for hot paths)

```go
type Foo struct{ Bar string }

func (f *Foo) Clone() *Foo {
    c := *f
    return &c
}

// clone.Clone[Foo] sees that *Foo implements Cloneable[Foo] and calls
// f.Clone() directly — the reflect path is skipped entirely.
c := clone.Clone(&Foo{Bar: "hello"})
```

### Pointer cycles via CloneSlowly

```go
type Node struct {
    Data int
    Self *Node
}

n := &Node{Data: 1}
n.Self = n  // self-reference

c := clone.CloneSlowly(n)
// c.Self == c (the cycle is preserved with independent identity)
// c is a fresh allocation, not aliased to n
```

`CloneSlowly` allocates a `visited` map per call, so concurrent callers don't share state. It does NOT consult `Cloneable[T]` — the reflect path with cycle detection always runs. Use `Clone` if you want the `Cloneable[T]` escape hatch and you know your graph is acyclic.

## Contract

- **Exported** pointer / slice / map / struct / interface / array fields are deep-cloned recursively.
- **Scalar exported fields and ALL unexported fields** are shallow-copied via a whole-struct `reflect.Value.Set`. This means `time.Time` and similar scalar-like structs round-trip correctly (wall + monotonic clock + location pointer all preserved). But structs whose **unexported** fields hold pointer-like data will share those values with the original — mutations to such shared values would be visible to both.
- **Pointer cycles** are not supported by `Clone` (will infinite-loop). Use `CloneSlowly`.
- **`sync.Mutex` / `sync.WaitGroup` and similar "no-copy" types** are not specially handled. Their unexported state is shallow-shared. Do not clone structs containing live mutexes.
- **`func` / `chan` / `unsafe.Pointer`** fields are shallow-copied (the closest sensible default for opaque references).

If `o == nil`, `Clone` and `CloneSlowly` both return a typed nil `*T`.

## Performance

Apple M5 Pro, Go 1.24.4, `go test -bench=. -benchmem -benchtime=2s -count=3`. Medians of 3 runs.

| Workload                                       |  huandu (ns) | clone-go (ns) | speed ratio                  | huandu allocs | clone-go allocs   |
| ---------------------------------------------- | -----------: | ------------: | ---------------------------- | ------------: | ----------------- |
| Simple struct (2 fields)                       |           53 |            47 | **0.89× (clone-go faster)**  |             1 | 1                 |
| Complex `map[string]*T`                        |          589 |           745 | 1.27× slower                 |            18 | 27                |
| Realistic `User` (slice + map + 2× time.Time)  |          395 |           711 | 1.80× slower                 |            12 | 18                |
| `*[]item` length=100                           |        5,214 |         5,163 | 1.00× (tie)                  |           103 | **3 (34× fewer)** |
| Binary tree depth=8 (511 nodes)                |       38,343 |        40,473 | 1.06× slower                 |           511 | 511               |
| Self-cycle via `CloneSlowly` / `huandu.Slowly` |          225 |            69 | **0.31× (clone-go 3.25×)**   |             4 | 1                 |

**Reading the table:**

- Simple struct and cycle path: **clone-go is faster**. huandu's per-type cache + allocator setup is overhead that doesn't pay off for tiny inputs; on the cycle path, huandu's full visited+invalid+fix-pass machinery is overhead for typical single-cycle inputs and our naive visited-map approach wins.
- Slice of 100 items: tied on speed, but clone-go uses **34× fewer allocations** (3 vs 103). One `reflect.MakeSlice` + per-element walk vs huandu's per-element allocator path. Real GC win for slice-heavy workloads.
- Tree of 511 nodes: ~6% slower. Recursion-heavy; reflect overhead is roughly the same as huandu's reflect-walk.
- Worst case is the User struct at 1.80× slower. Driven by absence of a per-type field cache. Fixable; deferred (see Roadmap).

To re-run on your machine:

```sh
cd benchmarks
go test -bench=. -benchmem -benchtime=2s -count=3
```

## Roadmap

- **Per-type field cache** in a `sync.Map` keyed by `reflect.Type`. Closes the 1.8× gap on the User-struct workload by skipping `reflect.StructField.IsExported` checks on subsequent calls. Add when profiling justifies it.
- **Scalar-slice fast path** via `reflect.Copy` for `[]int`, `[]byte`, `[]string`, etc. Skip per-element walk when element kind is scalar.
- **Optional `unsafe`-based unexported-field deep clone** behind a build tag, for users who need huandu-class fidelity for unexported pointer/slice/map fields. Default stays no-unsafe.
- **`MarkAsScalar`-style API** for downstream-supplied opaque types that should be shallow-copied as a unit.
- **Fuzz tests** with random struct generators to find unhandled `reflect.Kind` paths.

If any of these matter for your workload, open an issue with a reproducer.

## License

MIT — see [LICENSE](./LICENSE).
