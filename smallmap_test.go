package smallmap

import (
	"fmt"
	"sort"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func sorted[K ~string | ~int](keys []K) []K {
	out := make([]K, len(keys))
	copy(out, keys)
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]) < fmt.Sprint(out[j]) })
	return out
}

// assertLen fails the test if m.Len() != want.
func assertLen[K comparable, V any](t *testing.T, m *SmallMap[K, V], want int) {
	t.Helper()
	if got := m.Len(); got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}

// assertGet fails if Get(key) doesn't return (want, true).
func assertGet[K comparable, V comparable](t *testing.T, m *SmallMap[K, V], key K, want V) {
	t.Helper()
	got, ok := m.Get(key)
	if !ok {
		t.Errorf("Get(%v) not found, want %v", key, want)
		return
	}
	if got != want {
		t.Errorf("Get(%v) = %v, want %v", key, got, want)
	}
}

// assertAbsent fails if Get(key) returns a value (key should be missing).
func assertAbsent[K comparable, V any](t *testing.T, m *SmallMap[K, V], key K) {
	t.Helper()
	if _, ok := m.Get(key); ok {
		t.Errorf("Get(%v) found a value, expected absent", key)
	}
}

// ---- construction -----------------------------------------------------------

func TestNew(t *testing.T) {
	m := New[string, int]()
	if m == nil {
		t.Fatal("New returned nil")
	}
	assertLen(t, m, 0)
	if m.usingMap() {
		t.Error("new SmallMap should use array backend")
	}
}

// ---- basic Set / Get / Has --------------------------------------------------

func TestSetGetSingleEntry(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	assertLen(t, m, 1)
	assertGet(t, m, "a", 1)
	if !m.Has("a") {
		t.Error("Has(a) should be true")
	}
}

func TestSetOverwritesExistingKey(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("a", 99)
	assertLen(t, m, 1)
	assertGet(t, m, "a", 99)
}

func TestGetMissingKeyReturnsZero(t *testing.T) {
	m := New[string, int]()
	v, ok := m.Get("missing")
	if ok {
		t.Error("ok should be false for missing key")
	}
	if v != 0 {
		t.Errorf("value should be zero, got %v", v)
	}
}

func TestHasMissingKey(t *testing.T) {
	m := New[string, int]()
	if m.Has("nope") {
		t.Error("Has should return false for missing key")
	}
}

// ---- Delete -----------------------------------------------------------------

func TestDeleteExistingKey(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	if !m.Delete("a") {
		t.Error("Delete should return true for existing key")
	}
	assertLen(t, m, 1)
	assertAbsent(t, m, "a")
	assertGet(t, m, "b", 2)
}

func TestDeleteMissingKeyReturnsFalse(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	if m.Delete("nope") {
		t.Error("Delete should return false for missing key")
	}
	assertLen(t, m, 1)
}

func TestDeleteOnEmptyMap(t *testing.T) {
	m := New[string, int]()
	if m.Delete("nope") {
		t.Error("Delete on empty map should return false")
	}
}

// ---- Clear ------------------------------------------------------------------

func TestClear(t *testing.T) {
	m := New[string, int]()
	for i := range 5 {
		m.Set(fmt.Sprintf("key%d", i), i)
	}
	m.Clear()
	assertLen(t, m, 0)
	if m.usingMap() {
		t.Error("after Clear, SmallMap should not hold a map backend")
	}
	assertAbsent(t, m, "key0")
}

func TestClearAfterPromotion(t *testing.T) {
	m := New[string, int]()
	for i := range ArrayThreshold + 5 {
		m.Set(fmt.Sprintf("key%d", i), i)
	}
	if !m.usingMap() {
		t.Fatal("should be using map backend after filling beyond threshold")
	}
	m.Clear()
	assertLen(t, m, 0)
	if m.usingMap() {
		t.Error("after Clear, map backend should be released")
	}
}

func TestClearThenReuse(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Clear()
	m.Set("b", 2)
	assertLen(t, m, 1)
	assertGet(t, m, "b", 2)
	assertAbsent(t, m, "a")
}

// ---- promotion / demotion ---------------------------------------------------

func TestPromotionToMap(t *testing.T) {
	m := New[int, int]()
	for i := range ArrayThreshold {
		m.Set(i, i*10)
	}
	// At exactly ArrayThreshold the array is full; the next insert promotes.
	if m.usingMap() {
		t.Error("should still be using array at exactly ArrayThreshold entries")
	}
	m.Set(ArrayThreshold, ArrayThreshold*10)
	if !m.usingMap() {
		t.Error("should be using map backend after exceeding ArrayThreshold")
	}
	assertLen(t, m, ArrayThreshold+1)
	// Verify all values survived the promotion.
	for i := range ArrayThreshold + 1 {
		assertGet(t, m, i, i*10)
	}
}

func TestDemotionToArrayAfterDeletes(t *testing.T) {
	m := New[int, int]()
	total := ArrayThreshold + 10
	for i := range total {
		m.Set(i, i)
	}
	if !m.usingMap() {
		t.Fatal("should be in map mode")
	}
	// Delete entries until we drop to mapDemoteThreshold.
	target := mapDemoteThreshold
	for i := target; i < total; i++ {
		m.Delete(i)
	}
	if m.usingMap() {
		t.Errorf("should have demoted to array backend at %d entries (mapDemoteThreshold=%d)", m.Len(), mapDemoteThreshold)
	}
	assertLen(t, m, target)
	for i := range target {
		assertGet(t, m, i, i)
	}
}

func TestPromoteAfterClearAndRefill(t *testing.T) {
	m := New[int, int]()
	// Fill past threshold.
	for i := range ArrayThreshold + 5 {
		m.Set(i, i)
	}
	m.Clear()
	// Refill – should start array then promote again.
	for i := range ArrayThreshold + 5 {
		m.Set(i, i)
	}
	if !m.usingMap() {
		t.Error("should be in map mode after second fill")
	}
	assertLen(t, m, ArrayThreshold+5)
}

// ---- map recreation (mapRecreateThreshold) ----------------------------------

func TestMapRecreation(t *testing.T) {
	m := New[int, int]()
	// Promote to map.
	for i := range ArrayThreshold + 1 {
		m.Set(i, i)
	}
	if !m.usingMap() {
		t.Fatal("should be in map mode")
	}
	// Keep adding unique keys to trigger recreation.
	for i := ArrayThreshold + 1; i < mapRecreateThreshold+ArrayThreshold+2; i++ {
		m.Set(i, i)
	}
	// Map should still be functional (recreation is transparent).
	assertLen(t, m, mapRecreateThreshold+ArrayThreshold+2)
	for i := range mapRecreateThreshold + ArrayThreshold + 2 {
		assertGet(t, m, i, i)
	}
}

// ---- Range ------------------------------------------------------------------

func TestRangeArrayBackend(t *testing.T) {
	m := New[string, int]()
	want := map[string]int{"a": 1, "b": 2, "c": 3}
	for k, v := range want {
		m.Set(k, v)
	}
	got := map[string]int{}
	m.Range(func(k string, v int) bool {
		got[k] = v
		return true
	})
	for k, wv := range want {
		if gv, ok := got[k]; !ok || gv != wv {
			t.Errorf("Range: key %q got %d, want %d", k, gv, wv)
		}
	}
}

func TestRangeMapBackend(t *testing.T) {
	m := New[int, int]()
	for i := range ArrayThreshold + 5 {
		m.Set(i, i*2)
	}
	count := 0
	m.Range(func(k, v int) bool {
		if v != k*2 {
			t.Errorf("Range: key %d value %d, want %d", k, v, k*2)
		}
		count++
		return true
	})
	if count != ArrayThreshold+5 {
		t.Errorf("Range visited %d entries, want %d", count, ArrayThreshold+5)
	}
}

func TestRangeEarlyStop(t *testing.T) {
	m := New[int, int]()
	for i := range 10 {
		m.Set(i, i)
	}
	count := 0
	m.Range(func(_ int, _ int) bool {
		count++
		return count < 3
	})
	if count != 3 {
		t.Errorf("Range should stop at 3, got %d", count)
	}
}

// ---- Keys / Values ----------------------------------------------------------

func TestKeysArrayBackend(t *testing.T) {
	m := New[string, int]()
	m.Set("x", 1)
	m.Set("y", 2)
	m.Set("z", 3)
	keys := sorted(m.Keys())
	want := []string{"x", "y", "z"}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("Keys[%d] = %q, want %q", i, keys[i], k)
		}
	}
}

func TestValuesArrayBackend(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 10)
	m.Set("b", 20)
	values := m.Values()
	sum := 0
	for _, v := range values {
		sum += v
	}
	if sum != 30 {
		t.Errorf("Values sum = %d, want 30", sum)
	}
}

func TestKeysMapBackend(t *testing.T) {
	m := New[int, string]()
	for i := range ArrayThreshold + 3 {
		m.Set(i, fmt.Sprintf("v%d", i))
	}
	keys := m.Keys()
	if len(keys) != ArrayThreshold+3 {
		t.Errorf("Keys() len = %d, want %d", len(keys), ArrayThreshold+3)
	}
}

// ---- edge cases -------------------------------------------------------------

func TestSetAndGetNilValue(t *testing.T) {
	m := New[string, *int]()
	m.Set("ptr", nil)
	v, ok := m.Get("ptr")
	if !ok {
		t.Error("key should be present")
	}
	if v != nil {
		t.Error("value should be nil")
	}
}

func TestDeleteLastElement(t *testing.T) {
	m := New[string, int]()
	m.Set("only", 42)
	m.Delete("only")
	assertLen(t, m, 0)
	assertAbsent(t, m, "only")
}

func TestLargeInsertDeleteCycle(t *testing.T) {
	m := New[int, int]()
	const n = 200
	for i := range n {
		m.Set(i, i)
	}
	assertLen(t, m, n)
	for i := range n {
		m.Delete(i)
	}
	assertLen(t, m, 0)
	if m.usingMap() {
		t.Error("after full delete cycle SmallMap should not hold a map")
	}
}

func TestOverwriteInMapMode(t *testing.T) {
	m := New[int, int]()
	for i := range ArrayThreshold + 1 {
		m.Set(i, i)
	}
	m.Set(0, 999)
	assertGet(t, m, 0, 999)
	assertLen(t, m, ArrayThreshold+1)
}

func TestRangeEmptyMap(t *testing.T) {
	m := New[string, int]()
	called := false
	m.Range(func(_ string, _ int) bool {
		called = true
		return true
	})
	if called {
		t.Error("Range should not call fn on empty map")
	}
}
