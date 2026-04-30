// Package benchmarks compares github.com/pixie-sh/clone-go against
// github.com/huandu/go-clone/generic across representative workloads.
//
// Run with:
//
//	go test -bench=. -benchmem -benchtime=2s -count=3
//
// The benchmarks live in a sub-module so the main clone-go package keeps
// zero non-stdlib dependencies on the production side.
package benchmarks

import (
	"testing"
	"time"

	huandu "github.com/huandu/go-clone/generic"
	"github.com/pixie-sh/clone-go"
)

// ============================================================
// Workload 1: simple struct (matches huandu's BenchmarkSimpleClone)
// ============================================================

type simple struct {
	Foo int
	Bar string
}

func newSimple() *simple { return &simple{Foo: 123, Bar: "abcd"} }

func BenchmarkSimple_Huandu(b *testing.B) {
	o := newSimple()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Clone(o)
	}
}

func BenchmarkSimple_CloneGo(b *testing.B) {
	o := newSimple()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.Clone(o)
	}
}

// ============================================================
// Workload 2: complex map (matches huandu's BenchmarkComplexClone)
// ============================================================

type complexT struct {
	Foo int
	Bar map[string]interface{}
}

func newComplex() map[string]*complexT {
	return map[string]*complexT{
		"abc": {Foo: 123, Bar: map[string]interface{}{"abc": 321}},
		"def": {Foo: 456, Bar: map[string]interface{}{"def": 789}},
	}
}

func BenchmarkComplex_Huandu(b *testing.B) {
	o := newComplex()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Clone(o)
	}
}

func BenchmarkComplex_CloneGo(b *testing.B) {
	o := newComplex()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.Clone(&o)
	}
}

// ============================================================
// Workload 3: nested struct with slice + map + time
// (representative of typical core-go domain models)
// ============================================================

type address struct {
	Street string
	City   string
	Zip    string
}

type user struct {
	ID        int64
	Name      string
	Email     string
	Tags      []string
	Metadata  map[string]string
	Address   *address
	CreatedAt time.Time
	UpdatedAt time.Time
}

func newUser() *user {
	return &user{
		ID:    42,
		Name:  "Alice",
		Email: "alice@example.com",
		Tags:  []string{"admin", "verified", "premium"},
		Metadata: map[string]string{
			"plan":   "pro",
			"region": "eu-west",
			"tier":   "gold",
		},
		Address: &address{
			Street: "1 Infinite Loop",
			City:   "Cupertino",
			Zip:    "95014",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func BenchmarkUser_Huandu(b *testing.B) {
	o := newUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Clone(o)
	}
}

func BenchmarkUser_CloneGo(b *testing.B) {
	o := newUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.Clone(o)
	}
}

// ============================================================
// Workload 4: slice of structs (100 elements)
// ============================================================

type item struct {
	ID    int
	Label string
	Value float64
}

func newItems() *[]item {
	out := make([]item, 100)
	for i := range out {
		out[i] = item{ID: i, Label: "label", Value: float64(i) * 1.5}
	}
	return &out
}

func BenchmarkSlice100_Huandu(b *testing.B) {
	o := newItems()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Clone(o)
	}
}

func BenchmarkSlice100_CloneGo(b *testing.B) {
	o := newItems()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.Clone(o)
	}
}

// ============================================================
// Workload 5: deep tree (tests recursion overhead)
// ============================================================

type tree struct {
	Value int
	Left  *tree
	Right *tree
}

func makeTree(depth int) *tree {
	if depth == 0 {
		return &tree{Value: 1}
	}
	return &tree{
		Value: depth,
		Left:  makeTree(depth - 1),
		Right: makeTree(depth - 1),
	}
}

func BenchmarkTreeDepth8_Huandu(b *testing.B) {
	o := makeTree(8) // 511 nodes
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Clone(o)
	}
}

func BenchmarkTreeDepth8_CloneGo(b *testing.B) {
	o := makeTree(8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.Clone(o)
	}
}

// ============================================================
// Workload 6: cycle support — Slowly path
// ============================================================

type cnode struct {
	Data int
	Self *cnode
}

func newCycle() *cnode {
	n := &cnode{Data: 1}
	n.Self = n
	return n
}

func BenchmarkCycle_HuanduSlowly(b *testing.B) {
	o := newCycle()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = huandu.Slowly(o)
	}
}

func BenchmarkCycle_CloneGoSlowly(b *testing.B) {
	o := newCycle()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clone.CloneSlowly(o)
	}
}
