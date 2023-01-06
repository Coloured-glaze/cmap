package cmap

import (
	"strconv"
	"sync"
	"testing"
)

type Integer int

func (i Integer) String() string {
	return strconv.Itoa(int(i))
}

func BenchmarkItems(b *testing.B) {
	m := New[Animal]()

	// Insert 100 elements.
	for i := 0; i < 10000; i++ {
		m.Set(strconv.Itoa(i), Animal{strconv.Itoa(i)})
	}
	for i := 0; i < b.N; i++ {
		m.Items()
	}
}

func BenchmarkItemsInteger(b *testing.B) {
	m := NewStringer[Integer, Animal]()

	// Insert 100 elements.
	for i := 0; i < 10000; i++ {
		m.Set((Integer)(i), Animal{strconv.Itoa(i)})
	}
	for i := 0; i < b.N; i++ {
		m.Items()
	}
}
func directSharding(key uint32) uint32 {
	return key
}

func BenchmarkItemsInt(b *testing.B) {
	m := NewWithCustomShardingFunction[uint32, Animal](directSharding)

	// Insert 100 elements.
	for i := 0; i < 10000; i++ {
		m.Set((uint32)(i), Animal{strconv.Itoa(i)})
	}
	for i := 0; i < b.N; i++ {
		m.Items()
	}
}

func BenchmarkMarshalJson(b *testing.B) {
	m := New[Animal]()

	// Insert 100 elements.
	for i := 0; i < 10000; i++ {
		m.Set(strconv.Itoa(i), Animal{strconv.Itoa(i)})
	}
	for i := 0; i < b.N; i++ {
		_, err := m.MarshalJSON()
		if err != nil {
			b.FailNow()
		}
	}
}

func BenchmarkStrconv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strconv.Itoa(i)
	}
}

func BenchmarkSingleInsertAbsent(b *testing.B) {
	m := New[string]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(strconv.Itoa(i), "value")
	}
}

func BenchmarkSingleInsertAbsentSyncMap(b *testing.B) {
	var m sync.Map
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Store(strconv.Itoa(i), "value")
	}
}

// cpu: Intel(R) Core(TM) i3-10100F CPU @ 3.60GHz
// BenchmarkSingleInsertPresent-8   	 4071336	       291.6 ns/op	      48 B/op	       1 allocs/op
// BenchmarkSingleInsertPresent-8   	33900822	        35.37 ns/op	       0 B/op	       0 allocs/o
func BenchmarkSingleInsertPresent(b *testing.B) {
	m := New[string]()
	m.Set("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// go m.Set("key", "value")
		m.Set("key", "value")
	}
}

func BenchmarkSingleInsertPresentSyncMap(b *testing.B) {
	var m sync.Map
	m.Store("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Store("key", "value")
	}
}

func benchmarkMultiInsertDifferent(b *testing.B) {
	m := New[string]()
	finished := make(chan struct{}, b.N)
	_, set := GetSet(m, finished)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i), "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiInsertDifferentSyncMap(b *testing.B) {
	var m sync.Map
	finished := make(chan struct{}, b.N)
	_, set := GetSetSyncMap[string, string](&m, finished)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i), "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiInsertDifferent_1_Shard(b *testing.B) {
	runWithShards(benchmarkMultiInsertDifferent, b, 1)
}
func BenchmarkMultiInsertDifferent_16_Shard(b *testing.B) {
	runWithShards(benchmarkMultiInsertDifferent, b, 16)
}
func BenchmarkMultiInsertDifferent_32_Shard(b *testing.B) {
	runWithShards(benchmarkMultiInsertDifferent, b, 32)
}
func BenchmarkMultiInsertDifferent_256_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetDifferent, b, 256)
}

func BenchmarkMultiInsertSame(b *testing.B) {
	m := New[string]()
	finished := make(chan struct{}, b.N)
	_, set := GetSet(m, finished)
	m.Set("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set("key", "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiInsertSameSyncMap(b *testing.B) {
	var m sync.Map
	finished := make(chan struct{}, b.N)
	_, set := GetSetSyncMap[string, string](&m, finished)
	m.Store("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set("key", "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

// cpu: Intel(R) Core(TM) i3-10100F CPU @ 3.60GHz
// BenchmarkMultiGetSame-8   	 2392080	       498.2 ns/op	      16 B/op	       1 allocs/op
// BenchmarkMultiGetSame-8   	 4393612	       238.5 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMultiGetSame(b *testing.B) {
	m := New[string]()
	finished := make(chan struct{}, b.N)
	get, _ := GetSet(m, finished)
	m.Set("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go get("key", "value")
		// get("key", "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

// BenchmarkMultiGetSame2-8   	 4920494	       225.2 ns/op	      48 B/op	       1 allocs/op
// BenchmarkMultiGetSame2-8   	63326876	        19.45 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMultiGetSame2(b *testing.B) {
	m := New[string]()
	m.Set("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		//	go m.Get("key")
		m.Get("key")
	}
}

func BenchmarkMultiGetSameSyncMap(b *testing.B) {
	var m sync.Map
	finished := make(chan struct{}, b.N)
	get, _ := GetSetSyncMap[string, string](&m, finished)
	m.Store("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go get("key", "value")
	}
	for i := 0; i < b.N; i++ {
		<-finished
	}
}

// BenchmarkMultiGetSameSyncMap2-8   	 5505602	       216.3 ns/op	      32 B/op	       1 allocs/op
// BenchmarkMultiGetSameSyncMap2-8   	62824522	        18.80 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMultiGetSameSyncMap2(b *testing.B) {
	var m sync.Map
	m.Store("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// go m.Load("key")
		m.Load("key")
	}
}

// BenchmarkMultiGetSameMap-8   	 4691526	       254.8 ns/op	      49 B/op	       3 allocs/op
// BenchmarkMultiGetSameMap-8   	356029417	         3.365 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMultiGetSameMap(b *testing.B) {
	var m = make(map[string]string)
	m["key"] = ""
	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// go func() {
		// 	v, ok = m["key"]
		// }()
		_, ok = m["key"]
	}
	_ = ok
}

func benchmarkMultiGetSetDifferent(b *testing.B) {
	m := New[string]()
	finished := make(chan struct{}, 2*b.N)
	get, set := GetSet(m, finished)
	m.Set("-1", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i-1), "value")
		go get(strconv.Itoa(i), "value")
	}
	for i := 0; i < 2*b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiGetSetDifferentSyncMap(b *testing.B) {
	var m sync.Map
	finished := make(chan struct{}, 2*b.N)
	get, set := GetSetSyncMap[string, string](&m, finished)
	m.Store("-1", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i-1), "value")
		go get(strconv.Itoa(i), "value")
	}
	for i := 0; i < 2*b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiGetSetDifferent_1_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetDifferent, b, 1)
}
func BenchmarkMultiGetSetDifferent_16_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetDifferent, b, 16)
}
func BenchmarkMultiGetSetDifferent_32_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetDifferent, b, 32)
}
func BenchmarkMultiGetSetDifferent_256_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetDifferent, b, 256)
}

func benchmarkMultiGetSetBlock(b *testing.B) {
	m := New[string]()
	finished := make(chan struct{}, 2*b.N)
	get, set := GetSet(m, finished)
	for i := 0; i < b.N; i++ {
		m.Set(strconv.Itoa(i%100), "value")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i%100), "value")
		go get(strconv.Itoa(i%100), "value")
	}
	for i := 0; i < 2*b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiGetSetBlockSyncMap(b *testing.B) {
	var m sync.Map
	finished := make(chan struct{}, 2*b.N)
	get, set := GetSetSyncMap[string, string](&m, finished)
	for i := 0; i < b.N; i++ {
		m.Store(strconv.Itoa(i%100), "value")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go set(strconv.Itoa(i%100), "value")
		go get(strconv.Itoa(i%100), "value")
	}
	for i := 0; i < 2*b.N; i++ {
		<-finished
	}
}

func BenchmarkMultiGetSetBlock_1_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetBlock, b, 1)
}
func BenchmarkMultiGetSetBlock_16_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetBlock, b, 16)
}
func BenchmarkMultiGetSetBlock_32_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetBlock, b, 32)
}
func BenchmarkMultiGetSetBlock_256_Shard(b *testing.B) {
	runWithShards(benchmarkMultiGetSetBlock, b, 256)
}

func GetSet[K comparable, V any](m ConcurrentMap[K, V], finished chan struct{}) (set func(key K, value V), get func(key K, value V)) {
	return func(key K, value V) {
			for i := 0; i < 10; i++ {
				m.Get(key)
			}
			finished <- struct{}{}
		}, func(key K, value V) {
			for i := 0; i < 10; i++ {
				m.Set(key, value)
			}
			finished <- struct{}{}
		}
}

func GetSetSyncMap[K comparable, V any](m *sync.Map, finished chan struct{}) (get func(key K, value V), set func(key K, value V)) {
	get = func(key K, value V) {
		for i := 0; i < 10; i++ {
			m.Load(key)
		}
		finished <- struct{}{}
	}
	set = func(key K, value V) {
		for i := 0; i < 10; i++ {
			m.Store(key, value)
		}
		finished <- struct{}{}
	}
	return
}

func runWithShards(bench func(b *testing.B), b *testing.B, shardsCount int) {
	oldShardsCount := SHARD_COUNT
	SHARD_COUNT = shardsCount
	bench(b)
	SHARD_COUNT = oldShardsCount
}

func BenchmarkKeys(b *testing.B) {
	m := New[Animal]()

	// Insert 100 elements.
	for i := 0; i < 10000; i++ {
		m.Set(strconv.Itoa(i), Animal{strconv.Itoa(i)})
	}
	for i := 0; i < b.N; i++ {
		m.Keys()
	}
}
