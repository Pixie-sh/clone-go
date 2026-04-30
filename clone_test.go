package clone

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ----- helper types (test-only fixtures) -----

type person struct {
	Name string
	Age  int
}

type inner struct {
	X int
	Y string
}

type outer struct {
	Name  string
	Inner *inner
	Tags  []string
	Meta  map[string]int
	Items []inner
	Refs  map[string]*inner
}

type node struct {
	Data int
	Self *node
}

type mutualA struct {
	Name string
	B    *mutualB
}

type mutualB struct {
	Tag string
	A   *mutualA
}

// customCloner implements Cloneable[customCloner] so we can prove that Clone
// dispatches to it and that CloneSlowly does NOT.
type customCloner struct {
	Marker int
	calls  *int64 // pointer so the side-effect counter survives the *c value copy
}

func (c *customCloner) Clone() *customCloner {
	if c.calls != nil {
		atomic.AddInt64(c.calls, 1)
	}
	out := *c
	out.Marker = c.Marker + 1
	return &out
}

type ifaceHolder struct {
	V any
}

type timeWrapper struct {
	Name string
	At   time.Time
}

// ----- T012a: nil input -----

func TestClone_NilInput(t *testing.T) {
	assert.Nil(t, Clone[person](nil))
	assert.Nil(t, CloneSlowly[person](nil))
}

// ----- T012b: primitive pointer -----

func TestClone_PrimitivePointer(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		x := 42
		c := Clone(&x)
		assert.NotNil(t, c)
		assert.NotSame(t, &x, c)
		assert.Equal(t, 42, *c)
		*c = 99
		assert.Equal(t, 42, x, "mutating clone must not affect original")
	})
	t.Run("string", func(t *testing.T) {
		s := "hello"
		c := Clone(&s)
		assert.NotNil(t, c)
		assert.NotSame(t, &s, c)
		assert.Equal(t, "hello", *c)
		*c = "world"
		assert.Equal(t, "hello", s)
	})
}

// ----- T012c: flat struct -----

func TestClone_FlatStruct(t *testing.T) {
	p := &person{Name: "Alice", Age: 30}
	c := Clone(p)
	assert.NotSame(t, p, c)
	assert.Equal(t, p.Name, c.Name)
	assert.Equal(t, p.Age, c.Age)
	c.Name = "Bob"
	c.Age = 99
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, 30, p.Age)
}

// ----- T012d: nested struct pointer -----

func TestClone_NestedStructPointer(t *testing.T) {
	o := &outer{Name: "outer", Inner: &inner{X: 1, Y: "a"}}
	c := Clone(o)
	assert.NotSame(t, o, c)
	assert.NotNil(t, c.Inner)
	assert.NotSame(t, o.Inner, c.Inner)
	assert.Equal(t, 1, c.Inner.X)
	c.Inner.X = 999
	c.Inner.Y = "z"
	assert.Equal(t, 1, o.Inner.X)
	assert.Equal(t, "a", o.Inner.Y)
}

// ----- T012e: slice and map -----

func TestClone_SliceAndMap(t *testing.T) {
	o := &outer{
		Name: "n",
		Tags: []string{"a", "b"},
		Meta: map[string]int{"x": 1, "y": 2},
		Items: []inner{
			{X: 1, Y: "first"},
			{X: 2, Y: "second"},
		},
		Refs: map[string]*inner{
			"k": {X: 10, Y: "k"},
		},
	}
	c := Clone(o)

	c.Tags[0] = "MUT"
	assert.Equal(t, "a", o.Tags[0], "slice element mutation must not leak to original")

	c.Meta["x"] = 999
	assert.Equal(t, 1, o.Meta["x"], "map value mutation must not leak to original")

	c.Items[0].X = 999
	assert.Equal(t, 1, o.Items[0].X, "[]struct element mutation must not leak to original")

	assert.NotSame(t, o.Refs["k"], c.Refs["k"], "map[*T] values must be deep-cloned, not shared")
	c.Refs["k"].X = 999
	assert.Equal(t, 10, o.Refs["k"].X, "deep mutation through a map[*T] value must not leak")
}

// ----- T012f: Cloneable interface dispatch -----

func TestClone_CloneableDispatch(t *testing.T) {
	var counter int64
	c := &customCloner{Marker: 1, calls: &counter}
	out := Clone(c)
	assert.NotNil(t, out)
	assert.Equal(t, int64(1), atomic.LoadInt64(&counter), "Cloneable.Clone() must run exactly once")
	assert.Equal(t, 2, out.Marker, "Cloneable.Clone() must produce its own result, not the reflect path")
}

// ----- T012g: deep mutation independence -----

func TestClone_MutationIndependence(t *testing.T) {
	o := &outer{
		Name:  "o",
		Inner: &inner{X: 1, Y: "a"},
		Tags:  []string{"x", "y"},
		Meta:  map[string]int{"k": 5},
	}
	snapshot, err := json.Marshal(o)
	assert.NoError(t, err)

	c := Clone(o)
	c.Name = "c"
	c.Inner.X = 999
	c.Tags = append(c.Tags, "z")
	c.Tags[0] = "MUT"
	c.Meta["k"] = 999
	c.Meta["new"] = 1

	after, err := json.Marshal(o)
	assert.NoError(t, err)
	assert.JSONEq(t, string(snapshot), string(after), "original must be byte-equal to pre-clone snapshot")
}

// ----- T012h: time.Time round-trips correctly -----

func TestClone_TimePreserved(t *testing.T) {
	now := time.Now()
	w := &timeWrapper{Name: "evt", At: now}
	c := Clone(w)
	assert.NotSame(t, w, c)
	assert.True(t, c.At.Equal(now), "cloned time.Time must Equal the original")
	assert.Equal(t, time.Duration(0), c.At.Sub(now), "Sub returns zero (monotonic preserved by whole-struct Set)")
	assert.Equal(t, now.Location(), c.At.Location(), "location pointer must be preserved")
}

// ----- T012i: interface field with concrete type -----

func TestClone_InterfaceFieldConcreteType(t *testing.T) {
	in := &inner{X: 7, Y: "iface"}
	h := &ifaceHolder{V: in}
	c := Clone(h)
	assert.NotSame(t, h, c)
	got, ok := c.V.(*inner)
	assert.True(t, ok, "clone.V must still be *inner, not map[string]any or similar")
	assert.NotSame(t, in, got)
	assert.Equal(t, 7, got.X)
	got.X = 999
	assert.Equal(t, 7, in.X)
}

// ----- T012j: CloneSlowly basic -----

func TestCloneSlowly_BasicStruct(t *testing.T) {
	p := &person{Name: "Alice", Age: 30}
	c := CloneSlowly(p)
	assert.NotSame(t, p, c)
	assert.Equal(t, p.Name, c.Name)
	c.Name = "Bob"
	assert.Equal(t, "Alice", p.Name)
}

// ----- T012k: CloneSlowly does NOT consult Cloneable -----

func TestCloneSlowly_DoesNotCheckCloneable(t *testing.T) {
	var counter int64
	c := &customCloner{Marker: 1, calls: &counter}
	out := CloneSlowly(c)
	assert.NotNil(t, out)
	assert.Equal(t, int64(0), atomic.LoadInt64(&counter), "CloneSlowly must NOT invoke Cloneable.Clone()")
	assert.Equal(t, 1, out.Marker, "CloneSlowly produces a reflect-path clone, not the Cloneable result")
}

// ----- T012l: CloneSlowly handles a self-cycle -----

func TestCloneSlowly_HandlesSelfCycle(t *testing.T) {
	n := &node{Data: 1}
	n.Self = n
	c := CloneSlowly(n)
	assert.NotNil(t, c)
	assert.NotSame(t, n, c)
	assert.Equal(t, 1, c.Data)
	assert.NotNil(t, c.Self)
	assert.Same(t, c, c.Self, "self-cycle must close on the cloned node, not the original")
	assert.NotSame(t, n.Self, c.Self)
}

// ----- T012m: CloneSlowly handles a mutual cycle -----

func TestCloneSlowly_HandlesMutualCycle(t *testing.T) {
	a := &mutualA{Name: "a"}
	b := &mutualB{Tag: "b"}
	a.B = b
	b.A = a
	c := CloneSlowly(a)
	assert.NotNil(t, c)
	assert.NotSame(t, a, c)
	assert.NotNil(t, c.B)
	assert.NotSame(t, b, c.B)
	assert.NotNil(t, c.B.A)
	assert.Same(t, c, c.B.A, "mutual cycle: c.B.A must point back to c")
}
