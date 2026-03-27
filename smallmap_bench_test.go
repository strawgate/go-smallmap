package smallmap

import (
	"fmt"
	"testing"
)

// sizes used across all benchmark suites.
var benchSizes = []int{1, 5, 10, 20, 50, 100, 500}

// ---- Set --------------------------------------------------------------------

func BenchmarkSmallMap_Set(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := New[int, int]()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Clear()
				for j := range n {
					m.Set(j, j)
				}
			}
		})
	}
}

func BenchmarkGoMap_Set(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := make(map[int]int)
				for j := range n {
					m[j] = j
				}
			}
		})
	}
}

// ---- Get --------------------------------------------------------------------

func BenchmarkSmallMap_Get(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := New[int, int]()
			for j := range n {
				m.Set(j, j)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := range n {
					_, _ = m.Get(j)
				}
			}
		})
	}
}

func BenchmarkGoMap_Get(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := make(map[int]int, n)
			for j := range n {
				m[j] = j
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := range n {
					_, _ = m[j]
				}
			}
		})
	}
}

// ---- Delete -----------------------------------------------------------------

func BenchmarkSmallMap_Delete(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				m := New[int, int]()
				for j := range n {
					m.Set(j, j)
				}
				b.StartTimer()
				for j := range n {
					m.Delete(j)
				}
			}
		})
	}
}

func BenchmarkGoMap_Delete(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				m := make(map[int]int, n)
				for j := range n {
					m[j] = j
				}
				b.StartTimer()
				for j := range n {
					delete(m, j)
				}
			}
		})
	}
}

// ---- Clear ------------------------------------------------------------------

func BenchmarkSmallMap_Clear(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := New[int, int]()
			for j := range n {
				m.Set(j, j)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Clear()
			}
		})
	}
}

func BenchmarkGoMap_Clear(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := make(map[int]int, n)
			for j := range n {
				m[j] = j
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Equivalent: recreate the map (Go maps don't shrink via clear).
				m = make(map[int]int)
			}
		})
	}
}

// ---- Range ------------------------------------------------------------------

func BenchmarkSmallMap_Range(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := New[int, int]()
			for j := range n {
				m.Set(j, j)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Range(func(_ int, _ int) bool { return true })
			}
		})
	}
}

func BenchmarkGoMap_Range(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := make(map[int]int, n)
			for j := range n {
				m[j] = j
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for range m {
				}
			}
		})
	}
}

// ---- Threshold tuning -------------------------------------------------------
// BenchmarkThresholdTuning runs Set+Get workloads at various sizes with the
// SmallMap to help identify the optimal ArrayThreshold value.

func BenchmarkThresholdTuning(b *testing.B) {
	sizes := []int{5, 8, 10, 15, 20, 25, 30, 40}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("SmallMap/n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := New[int, int]()
				for j := range n {
					m.Set(j, j)
				}
				for j := range n {
					_, _ = m.Get(j)
				}
			}
		})
		b.Run(fmt.Sprintf("GoMap/n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := make(map[int]int)
				for j := range n {
					m[j] = j
				}
				for j := range n {
					_, _ = m[j]
				}
			}
		})
	}
}

// ---- Memory alloc comparison ------------------------------------------------

func BenchmarkSmallMap_SetSmall_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := New[string, int]()
		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)
		_ = m
	}
}

func BenchmarkGoMap_SetSmall_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := make(map[string]int)
		m["a"] = 1
		m["b"] = 2
		m["c"] = 3
		_ = m
	}
}
