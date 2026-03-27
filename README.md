# go-smallmap

A generic key-value store for Go that avoids the overhead of Go's built-in maps for small collections.

## Motivation

Go maps are heap-allocated and their internal buckets **never shrink** after deletions. This creates
two problems for short-lived or pooled code:

1. **Creation cost** – every `make(map[K]V)` costs at least ~50 bytes and a heap allocation, even for maps that hold only a handful of entries.
2. **Memory retention** – `clear(m)` or `delete(m, key)` do not release bucket memory, so pooled objects that contain maps grow without bound over time.

`SmallMap` solves both problems with a **dual-backend design**:

| Condition | Backend | Allocations |
|-----------|---------|-------------|
| ≤ 20 entries | flat array (linear scan) | 0 (after first `New`) |
| > 20 entries | native Go map | 1 |
| After `Clear()` | flat array (capacity retained) | 0 |

## Usage

```go
import "github.com/strawgate/go-smallmap"

m := smallmap.New[string, int]()

m.Set("foo", 42)
v, ok := m.Get("foo")   // 42, true
m.Has("foo")            // true
m.Len()                 // 1

m.Range(func(k string, v int) bool {
    fmt.Println(k, v)
    return true // return false to stop early
})

m.Delete("foo")
m.Clear()               // releases any map backend; array capacity retained
```

## API

| Method | Description |
|--------|-------------|
| `New[K, V]()` | Create a new, empty SmallMap |
| `Set(key, value)` | Insert or update a key-value pair |
| `Get(key) (V, bool)` | Retrieve a value and whether the key was present |
| `Has(key) bool` | Report whether key exists |
| `Delete(key) bool` | Remove a key; returns true if it existed |
| `Len() int` | Number of entries |
| `Range(func(K,V) bool)` | Iterate over all entries; return false to stop early |
| `Clear()` | Remove all entries and release the map backend |
| `Keys() []K` | Snapshot of all keys |
| `Values() []V` | Snapshot of all values |

## Benchmarks

Run `go test -bench=. -benchmem ./...` to see results for your platform. Representative numbers on an AMD EPYC 7763 (array threshold = 20):

```
BenchmarkSmallMap_Set/n=1     5.6 ns/op    0 B/op   0 allocs/op
BenchmarkGoMap_Set/n=1       20.4 ns/op    0 B/op   0 allocs/op

BenchmarkSmallMap_Set/n=10   68.9 ns/op    0 B/op   0 allocs/op
BenchmarkGoMap_Set/n=10     474   ns/op  328 B/op   3 allocs/op

BenchmarkSmallMap_Set/n=20  180   ns/op    0 B/op   0 allocs/op
BenchmarkGoMap_Set/n=20    1157   ns/op  936 B/op   5 allocs/op
```

SmallMap is **6–7× faster** for ≤ 20-entry workloads and allocates zero bytes in the steady state (after the initial `New()`).

## Thresholds

| Constant | Value | Purpose |
|----------|-------|---------|
| `ArrayThreshold` | 20 | Switch from array to map when entries exceed this |
| `mapDemoteThreshold` | 10 | Switch from map back to array when entries drop to this |
| `mapRecreateThreshold` | 64 | Recreate the map after this many unique inserts (reclaims deleted-bucket memory) |
