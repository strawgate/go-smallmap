// Package smallmap provides SmallMap, a generic key-value store that uses
// a flat array for small collections and automatically promotes to a native
// Go map when the collection grows beyond a configurable threshold.
//
// Motivation: Go maps are heap-allocated and never shrink their internal
// buckets. For short-lived or frequently-cleared maps with a small number
// of entries this creates significant memory pressure. SmallMap avoids map
// allocation entirely while the entry count stays below ArrayThreshold, and
// reclaims the map allocation on every Clear() call so pooled objects return
// to a zero-allocation idle state.
package smallmap

// ArrayThreshold is the maximum number of entries stored in the flat-array
// backend. Once a Set() call would push the count above this value the
// entries are migrated to a native Go map. Chosen empirically via the
// package benchmarks; see BenchmarkThresholdTuning.
const ArrayThreshold = 20

// mapRecreateThreshold is the number of write operations (inserts + updates)
// performed against the map backend before we recreate the underlying Go map.
// Go maps never return bucket memory to the runtime after deletes, so periodic
// recreation prevents unbounded memory retention in long-lived SmallMaps that
// cycle through many inserts and deletes.
const mapRecreateThreshold = 64

// mapDemoteThreshold is the maximum entry count at which we convert the map
// backend back to the array backend after a batch of deletes. This is set to
// half of ArrayThreshold to avoid flip-flopping near the boundary.
const mapDemoteThreshold = ArrayThreshold / 2

// SmallMap is a generic key-value store compatible with the public interface
// of Go's built-in map type. It is NOT safe for concurrent use; callers must
// provide their own synchronisation.
type SmallMap[K comparable, V any] struct {
	// Array backend – used when len ≤ ArrayThreshold.
	arrayKeys   []K
	arrayValues []V

	// Map backend – non-nil only when len > ArrayThreshold.
	mapBackend map[K]V

	// mapWrites counts inserts and updates since the last map creation.
	// When it exceeds mapRecreateThreshold the map is recreated to release
	// memory held by deleted buckets.
	mapWrites int
}

// New returns a new, empty SmallMap with array storage pre-allocated.
func New[K comparable, V any]() *SmallMap[K, V] {
	return &SmallMap[K, V]{
		arrayKeys:   make([]K, 0, ArrayThreshold),
		arrayValues: make([]V, 0, ArrayThreshold),
	}
}

// usingMap reports whether the map backend is currently active.
func (m *SmallMap[K, V]) usingMap() bool {
	return m.mapBackend != nil
}

// arrayFind returns the index of key in the array backend, or -1 if absent.
func (m *SmallMap[K, V]) arrayFind(key K) int {
	for i, k := range m.arrayKeys {
		if k == key {
			return i
		}
	}
	return -1
}

// promoteToMap migrates all array entries into a new Go map and clears the
// array slices (retaining their underlying capacity for future demotions).
func (m *SmallMap[K, V]) promoteToMap() {
	m.mapBackend = make(map[K]V, ArrayThreshold*2)
	for i, k := range m.arrayKeys {
		m.mapBackend[k] = m.arrayValues[i]
	}
	m.mapWrites = len(m.mapBackend)
	m.arrayKeys = m.arrayKeys[:0]
	m.arrayValues = m.arrayValues[:0]
}

// demoteToArray migrates all map entries back into the array backend and
// releases the map so it can be garbage-collected.
func (m *SmallMap[K, V]) demoteToArray() {
	for k, v := range m.mapBackend {
		m.arrayKeys = append(m.arrayKeys, k)
		m.arrayValues = append(m.arrayValues, v)
	}
	m.mapBackend = nil
	m.mapWrites = 0
}

// recreateMap rebuilds the Go map from its current contents to release memory
// retained by previously deleted buckets, then resets the write counter.
func (m *SmallMap[K, V]) recreateMap() {
	fresh := make(map[K]V, len(m.mapBackend)*2)
	for k, v := range m.mapBackend {
		fresh[k] = v
	}
	m.mapBackend = fresh
	m.mapWrites = 0
}

// Set inserts or updates the value associated with key.
func (m *SmallMap[K, V]) Set(key K, value V) {
	if m.usingMap() {
		_, exists := m.mapBackend[key]
		m.mapBackend[key] = value
		if !exists {
			m.mapWrites++
			if m.mapWrites >= mapRecreateThreshold {
				m.recreateMap()
			}
		}
		return
	}

	// Update existing array entry.
	if idx := m.arrayFind(key); idx >= 0 {
		m.arrayValues[idx] = value
		return
	}

	// Array has room – append.
	if len(m.arrayKeys) < ArrayThreshold {
		m.arrayKeys = append(m.arrayKeys, key)
		m.arrayValues = append(m.arrayValues, value)
		return
	}

	// Array is full – promote to map and insert.
	m.promoteToMap()
	m.mapBackend[key] = value
	m.mapWrites++
}

// Get returns the value associated with key and whether the key was present.
func (m *SmallMap[K, V]) Get(key K) (V, bool) {
	if m.usingMap() {
		v, ok := m.mapBackend[key]
		return v, ok
	}
	if idx := m.arrayFind(key); idx >= 0 {
		return m.arrayValues[idx], true
	}
	var zero V
	return zero, false
}

// Has reports whether key is present in the map.
func (m *SmallMap[K, V]) Has(key K) bool {
	if m.usingMap() {
		_, ok := m.mapBackend[key]
		return ok
	}
	return m.arrayFind(key) >= 0
}

// Delete removes key from the map and returns true if the key was present.
// If the map backend shrinks below mapDemoteThreshold after the delete,
// the entries are migrated back to the array backend. If mapWrites exceeds
// mapRecreateThreshold (indicating heavy insert/delete churn), the map is
// recreated to release memory from deleted buckets.
func (m *SmallMap[K, V]) Delete(key K) bool {
	if m.usingMap() {
		if _, exists := m.mapBackend[key]; !exists {
			return false
		}
		delete(m.mapBackend, key)
		switch {
		case len(m.mapBackend) <= mapDemoteThreshold:
			m.demoteToArray()
		case m.mapWrites >= mapRecreateThreshold:
			// Heavy churn: many inserts were later deleted; recreate to
			// release bucket memory retained by the Go runtime.
			m.recreateMap()
		}
		return true
	}

	idx := m.arrayFind(key)
	if idx < 0 {
		return false
	}
	last := len(m.arrayKeys) - 1
	m.arrayKeys[idx] = m.arrayKeys[last]
	m.arrayValues[idx] = m.arrayValues[last]
	// Zero out the vacated tail slots to release references.
	var zeroK K
	var zeroV V
	m.arrayKeys[last] = zeroK
	m.arrayValues[last] = zeroV
	m.arrayKeys = m.arrayKeys[:last]
	m.arrayValues = m.arrayValues[:last]
	return true
}

// Len returns the number of entries currently stored in the SmallMap.
func (m *SmallMap[K, V]) Len() int {
	if m.usingMap() {
		return len(m.mapBackend)
	}
	return len(m.arrayKeys)
}

// Range calls fn for each key-value pair in the SmallMap. Iteration stops
// early if fn returns false, mirroring the behaviour of range over a map.
// The iteration order is not guaranteed to be consistent.
func (m *SmallMap[K, V]) Range(fn func(K, V) bool) {
	if m.usingMap() {
		for k, v := range m.mapBackend {
			if !fn(k, v) {
				return
			}
		}
		return
	}
	for i, k := range m.arrayKeys {
		if !fn(k, m.arrayValues[i]) {
			return
		}
	}
}

// Clear removes all entries and releases the map backend (if any), returning
// the SmallMap to its zero-allocation idle state.  The array slices retain
// their capacity so subsequent inserts do not allocate.
func (m *SmallMap[K, V]) Clear() {
	// Zero out array entries to release any held references.
	var zeroK K
	var zeroV V
	for i := range m.arrayKeys {
		m.arrayKeys[i] = zeroK
		m.arrayValues[i] = zeroV
	}
	m.arrayKeys = m.arrayKeys[:0]
	m.arrayValues = m.arrayValues[:0]
	m.mapBackend = nil
	m.mapWrites = 0
}

// Keys returns a snapshot of all keys in the SmallMap. The order is not
// guaranteed to be consistent.
func (m *SmallMap[K, V]) Keys() []K {
	if m.usingMap() {
		keys := make([]K, 0, len(m.mapBackend))
		for k := range m.mapBackend {
			keys = append(keys, k)
		}
		return keys
	}
	result := make([]K, len(m.arrayKeys))
	copy(result, m.arrayKeys)
	return result
}

// Values returns a snapshot of all values in the SmallMap. The order is not
// guaranteed to be consistent.
func (m *SmallMap[K, V]) Values() []V {
	if m.usingMap() {
		values := make([]V, 0, len(m.mapBackend))
		for _, v := range m.mapBackend {
			values = append(values, v)
		}
		return values
	}
	result := make([]V, len(m.arrayValues))
	copy(result, m.arrayValues)
	return result
}
