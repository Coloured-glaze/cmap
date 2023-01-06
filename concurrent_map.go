package cmap

import (
	"fmt"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var (
	SHARD_COUNT = 32 // 默认map分片数量
	json        = jsoniter.ConfigCompatibleWithStandardLibrary
)

type Stringer interface {
	fmt.Stringer
	comparable
}

// A "thread" safe map of type string:Anything.
// To avoid lock bottlenecks this map is dived to several (SHARD_COUNT) map shards.
//
// 一个 "线程" 安全的map 类型为 string:Anything
// 为了避免锁瓶颈，该map被划分为多个(SHARD_COUNT) 分片计数map。
type ConcurrentMap[K comparable, V any] struct {
	shards   []*ConcurrentMapShared[K, V] // map分片
	sharding func(key K) uint32           // 分片
}

// A "thread" safe string to anything map.
//
// 一个key为string的线程安全的任意map
type ConcurrentMapShared[K comparable, V any] struct {
	items        map[K]V // 内部map分片
	sync.RWMutex         // 读写锁保护对内部map的访问.
}

// Creates a new concurrent map.
//
// 创建新的并发map
func create[K comparable, V any](sharding func(key K) uint32) ConcurrentMap[K, V] {
	m := ConcurrentMap[K, V]{
		sharding: sharding,
		shards:   make([]*ConcurrentMapShared[K, V], SHARD_COUNT),
	}
	for i := 0; i < SHARD_COUNT; i++ {
		m.shards[i] = &ConcurrentMapShared[K, V]{items: make(map[K]V)}
	}
	return m
}

// Creates a new concurrent map.
//
// 创建新的并发map
func New[V any]() ConcurrentMap[string, V] {
	return create[string, V](fnv32)
}

// Creates a new concurrent map.
//
// 创建新的并发map
func NewStringer[K Stringer, V any]() ConcurrentMap[K, V] {
	return create[K, V](strfnv32[K])
}

// Creates a new concurrent map.
//
// 创建新的并发map
func NewWithCustomShardingFunction[K comparable, V any](sharding func(key K) uint32) ConcurrentMap[K, V] {
	return create[K, V](sharding)
}

// Get map shard
//
// 获取map分片
func (cms *ConcurrentMapShared[K, V]) GetMap() map[K]V {
	return cms.items
}

// GetShard returns shard under given key
//
// GetShard 返回给定key下的map分片
func (m ConcurrentMap[K, V]) GetShard(key K) *ConcurrentMapShared[K, V] {
	return m.shards[uint(m.sharding(key))%uint(SHARD_COUNT)]
}

func (m ConcurrentMap[K, V]) MSet(data map[K]V) {
	for key, value := range data {
		shard := m.GetShard(key)
		shard.Lock()
		shard.items[key] = value
		shard.Unlock()
	}
}

// Sets the given value under the specified key.
//
// 设置指定key下的给定值。
func (m ConcurrentMap[K, V]) Set(key K, value V) {
	// Get map shard.
	shard := m.GetShard(key)
	shard.Lock()
	shard.items[key] = value
	shard.Unlock()
}

// Callback to return new element to be inserted into the map
// It is called while lock is held, therefore it MUST NOT
// try to access other keys in same map, as it can lead to deadlock since
// Go sync.RWLock is not reentrant
//
// 回调以返回要插入到map中的新元素
// 它在锁定时被调用，因此不能
// 尝试访问同一map中的其他key，因为这可能会导致死锁，因为
// Go sync.RWLock 不可重入
type UpsertCb[V any] func(exist bool, valueInMap V, newValue V) V

// Insert or Update - updates existing element or inserts a new one using UpsertCb
//
// Insert 或 Update - 使用 UpsertCb 更新现有元素或插入新元素
func (m ConcurrentMap[K, V]) Upsert(key K, value V, cb UpsertCb[V]) (res V) {
	shard := m.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	res = cb(ok, v, value)
	shard.items[key] = res
	shard.Unlock()
	return res
}

// Sets the given value under the specified key if no value was associated with it.
//
// 如果没有值与指定键关联，则在指定键下设置给定值。
func (m ConcurrentMap[K, V]) SetIfAbsent(key K, value V) bool {
	// Get map shard.
	shard := m.GetShard(key)
	shard.Lock()
	_, ok := shard.items[key]
	if !ok {
		shard.items[key] = value
	}
	shard.Unlock()
	return !ok
}

// Get retrieves an element from map under given key.
//
// Get 从给定key下的映射中检索元素。
func (m ConcurrentMap[K, V]) Get(key K) (V, bool) {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// Get item from shard.
	val, ok := shard.items[key]
	shard.RUnlock()
	return val, ok
}

// Count returns the number of elements within the map.
//
// Count返回map中元素的数量。
func (m ConcurrentMap[K, V]) Count() int {
	count := 0
	for i := 0; i < SHARD_COUNT; i++ {
		shard := m.shards[i]
		shard.RLock()
		count += len(shard.items)
		shard.RUnlock()
	}
	return count
}

// Looks up an item under specified key
//
// 查找指定key下的项目
func (m ConcurrentMap[K, V]) Has(key K) bool {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// See if element is within shard.
	_, ok := shard.items[key]
	shard.RUnlock()
	return ok
}

// Remove removes an element from the map.
//
// Remove 从map中移除指定元素
func (m ConcurrentMap[K, V]) Remove(key K) {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	delete(shard.items, key)
	shard.Unlock()
}

// RemoveCb is a callback executed in a map.RemoveCb() call, while Lock is held
// If returns true, the element will be removed from the map
//
// RemoveCb是在map中执行的回调。RemoveCb()调用，而Lock被保持
// 如果返回true，元素将从map中删除
type RemoveCb[K any, V any] func(key K, v V, exists bool) bool

// RemoveCb locks the shard containing the key, retrieves its current value and calls the callback with those params
// If callback returns true and element exists, it will remove it from the map
// Returns the value returned by the callback (even if element was not present in the map)
//
// RemoveCb锁定包含key的分片，检索其当前值，并使用这些参数调用回调
// 如果回调返回true并且元素存在，它将从map中删除它
// 返回回调返回的值（即使元素不在map中）
func (m ConcurrentMap[K, V]) RemoveCb(key K, cb RemoveCb[K, V]) bool {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	remove := cb(key, v, ok)
	if remove && ok {
		delete(shard.items, key)
	}
	shard.Unlock()
	return remove
}

// Pop removes an element from the map and returns it
//
// Pop从map中删除元素并将其返回
func (m ConcurrentMap[K, V]) Pop(key K) (v V, exists bool) {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	v, exists = shard.items[key]
	delete(shard.items, key)
	shard.Unlock()
	return v, exists
}

// IsEmpty checks if map is empty.
//
// IsEmpty检查map是否为空。
func (m ConcurrentMap[K, V]) IsEmpty() bool {
	return m.Count() == 0
}

// Used by the Iter & IterBuffered functions to wrap two variables together over a channel,
//
// 由 Iter & IterBuffered 函数用于将两个变量包装在一个管道上，
type Tuple[K comparable, V any] struct {
	Key K
	Val V
}

// Iter returns an iterator which could be used in a for range loop.
// Deprecated: using IterBuffered() will get a better performence
//
// Iter 返回一个迭代器，可以在for range循环中使用。
// 不推荐：使用 IterBuffered() 将获得更好的性能
func (m ConcurrentMap[K, V]) Iter() <-chan Tuple[K, V] {
	chans := snapshot(m)
	ch := make(chan Tuple[K, V])
	go fanIn(chans, ch)
	return ch
}

// IterBuffered returns a buffered iterator which could be used in a for range loop.
//
// IterBuffered 返回一个缓冲迭代器，可以在for range循环中使用。
func (m ConcurrentMap[K, V]) IterBuffered() <-chan Tuple[K, V] {
	chans := snapshot(m)
	total := 0
	for _, c := range chans {
		total += cap(c)
	}
	ch := make(chan Tuple[K, V], total)
	go fanIn(chans, ch)
	return ch
}

// Clear removes all items from map.
//
// Clear 将从map中删除所有项目。
func (m ConcurrentMap[K, V]) Clear() {
	for item := range m.IterBuffered() {
		m.Remove(item.Key)
	}
}

// Returns a array of channels that contains elements in each shard,
// which likely takes a snapshot of `m`.
// It returns once the size of each buffered channel is determined,
// before all the channels are populated using goroutines.
//
// 返回一个管道数组，其中包含每个碎片中的元素，这可能会获取“m”的快照。
// 在使用goroutine填充所有管道之前，一旦确定了每个缓冲管道的大小，它就会return。
func snapshot[K comparable, V any](m ConcurrentMap[K, V]) (chans []chan Tuple[K, V]) {
	// When you access map items before initializing.
	// 当访问映射时，初始化之前的项
	if len(m.shards) == 0 {
		panic(`cmap.ConcurrentMap is not initialized. Should run New() before usage.`)
	}
	chans = make([]chan Tuple[K, V], SHARD_COUNT)
	wg := sync.WaitGroup{}
	wg.Add(SHARD_COUNT)
	// Foreach shard.
	for index, shard := range m.shards {
		go func(index int, shard *ConcurrentMapShared[K, V]) {
			// Foreach key, value pair.
			shard.RLock()
			chans[index] = make(chan Tuple[K, V], len(shard.items))
			wg.Done()
			for key, val := range shard.items {
				chans[index] <- Tuple[K, V]{key, val}
			}
			shard.RUnlock()
			close(chans[index])
		}(index, shard)
	}
	wg.Wait()
	return chans
}

// fanIn reads elements from channels `chans` into channel `out`
//
// fanIn 将元素从管道 `chans` 读入管道 `out`
func fanIn[K comparable, V any](chans []chan Tuple[K, V], out chan Tuple[K, V]) {
	wg := sync.WaitGroup{}
	wg.Add(len(chans))
	for _, ch := range chans {
		go func(ch chan Tuple[K, V]) {
			for t := range ch {
				out <- t
			}
			wg.Done()
		}(ch)
	}
	wg.Wait()
	close(out)
}

// Items returns all items as map[string]V
//
// Items 将所有项目返回为 map[string]V
func (m ConcurrentMap[K, V]) Items() map[K]V {
	tmp := make(map[K]V)

	// Insert items to temporary map.
	// 将项目插入临时map。
	for item := range m.IterBuffered() {
		tmp[item.Key] = item.Val
	}
	return tmp
}

// Iterator callbacalled for every key,value found in
// maps. RLock is held for all calls for a given shard
// therefore callback sess consistent view of a shard,
// but not across the shards
//
// 为map中找到的每个键、值调用迭代器。
// RLock用于给定碎片的所有调用，因此回调会获得分片的一致视图，但不会跨越分片
type IterCb[K comparable, V any] func(key K, v V)

// Callback based iterator, cheapest way to read all elements in a map.
//
// 基于回调的迭代器，读取map中所有元素的最简易方法
func (m ConcurrentMap[K, V]) IterCb(fn IterCb[K, V]) {
	for idx := range m.shards {
		shard := (m.shards)[idx]
		shard.RLock()
		for key, value := range shard.items {
			fn(key, value)
		}
		shard.RUnlock()
	}
}

// Keys returns all keys as []string
//
// Keys 将所有key返回 []string
func (m ConcurrentMap[K, V]) Keys() []K {
	count := m.Count()
	ch := make(chan K, count)
	go func() {
		// Foreach shard.
		wg := sync.WaitGroup{}
		wg.Add(SHARD_COUNT)
		for _, shard := range m.shards {
			go func(shard *ConcurrentMapShared[K, V]) {
				// Foreach key, value pair.
				shard.RLock()
				for key := range shard.items {
					ch <- key
				}
				shard.RUnlock()
				wg.Done()
			}(shard)
		}
		wg.Wait()
		close(ch)
	}()

	// Generate keys
	// 生成 key
	keys := make([]K, 0, count)
	for k := range ch {
		keys = append(keys, k)
	}
	return keys
}

func strfnv32[K fmt.Stringer](key K) uint32 {
	return fnv32(key.String())
}

// 对字符串进行哈希求值
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	keyLength := len(key)
	for i := 0; i < keyLength; i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return hash
}

// Reviles ConcurrentMap "private" variables to json marshal.
//
// 将 ConcurrentMap 序列化为json.
func (m ConcurrentMap[K, V]) MarshalJSON() ([]byte, error) {
	// Create a temporary map, which will hold all item spread across shards.
	// 创建一个临时map，它将保存所有分片上的项目
	tmp := make(map[K]V)

	// Insert items to temporary map.
	// 将项目插入临时map
	for item := range m.IterBuffered() {
		tmp[item.Key] = item.Val
	}
	return json.Marshal(tmp)
}

// Reverse process of Marshal.
//
// 反序列化json
func (m *ConcurrentMap[K, V]) UnmarshalJSON(b []byte) (err error) {
	tmp := make(map[K]V)

	// Unmarshal into a single map.
	// json反序列化到map
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	// foreach key,value pair in temporary map insert into our concurrent map.
	// 临时map中的值对插入到并发map中。
	for key, val := range tmp {
		m.Set(key, val)
	}
	return nil
}
