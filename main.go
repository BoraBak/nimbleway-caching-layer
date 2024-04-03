package main

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"
)

type CacheItem struct {
	key   int
	value []byte
}

type ARC struct {
	cache            map[int]*list.Element
	lru, mru         *list.List
	lruSize, mruSize int
	maxSize          int
	mu               sync.Mutex
	file             *DatabaseFile
}

type DatabaseFile struct {
	// Add necessary fields and methods to interact with file.db
}

func (df *DatabaseFile) ReadDataBlock(key int) ([]byte, error) {
	return nil, errors.New("testing purposes")
}

func NewARC(maxSize int, dbFile *DatabaseFile) *ARC {
	return &ARC{
		cache:   make(map[int]*list.Element),
		lru:     list.New(),
		mru:     list.New(),
		lruSize: 0,
		mruSize: 0,
		maxSize: maxSize,
		file:    dbFile,
	}
}

func (c *ARC) Get(key int) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	var result []byte
	if item, isKeyExistsInCache := c.cache[key]; isKeyExistsInCache {
		result = c.handleCacheHit(item)
	} else {
		result = c.handleCacheMiss(key)
	}

	return result
}

func (c *ARC) handleCacheHit(item *list.Element) []byte {
	c.mru.MoveToFront(item)
	return item.Value.(*CacheItem).value
}

func (c *ARC) handleCacheMiss(key int) []byte {
	item := c.getDataFromDB(key)
	c.addNewItemToLRU(key, item)
	return item
}

func (c *ARC) Set(key int, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, isKeyExistsInCache := c.cache[key]; isKeyExistsInCache {
		return
	}

	if c.isCacheReachedMaxSize() {
		c.evict()
	}

	c.addNewItemToMRU(key, value)
}

func (c *ARC) addNewItemToLRU(key int, value []byte) {
	newItem := &CacheItem{
		key:   key,
		value: value,
	}

	element := c.lru.PushFront(newItem)
	c.cache[key] = element
	c.lruSize++
}

func (c *ARC) addNewItemToMRU(key int, value []byte) {
	newItem := &CacheItem{
		key:   key,
		value: value,
	}

	element := c.mru.PushFront(newItem)
	c.cache[key] = element
	c.mruSize++
}

func (c *ARC) isCacheReachedMaxSize() bool {
	return c.lruSize+c.mruSize >= c.maxSize
}

// evict evicts items from LRU until the size constraint is satisfied
func (c *ARC) evict() {
	if c.isCacheReachedMaxSize() {
		totalSize := c.lruSize + c.mruSize
		for c.lru.Len() > 0 && totalSize >= c.maxSize {
			evicted := c.lru.Back()
			if evicted == nil {
				break
			}
			evictedItem := evicted.Value.(*CacheItem)
			delete(c.cache, evictedItem.key)
			c.lru.Remove(evicted)
			c.lruSize--
			totalSize--
		}
	}
}

func (c *ARC) getDataFromDB(key int) []byte {
	data, err := c.file.ReadDataBlock(key)
	if err != nil {
		return nil
	}

	c.Set(key, data)

	return data
}

func main() {
	dbFile := &DatabaseFile{}
	arc := NewARC(10*1024*1024, dbFile)

	simulate8KBRead(arc)
	simulate64KBReadAfter2secDelay(arc)
	simulateWriteOperation(arc)

	fmt.Println("Data written to cache")
	time.Sleep(5 * time.Second)
}

func simulate8KBRead(arc *ARC) {
	go func() {
		for i := 0; i < 100; i++ {
			data := arc.Get(i)
			if data == nil {
				fmt.Println("[x] Cache miss for key", i)
			} else {
				fmt.Println("[v] Cache hit for key", i)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func simulate64KBReadAfter2secDelay(arc *ARC) {
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(2 * time.Second)
			data := arc.Get(i)
			if data == nil {
				fmt.Println("[x] Cache miss for key", i)
			} else {
				fmt.Println("[v] Cache hit for key", i)
			}
		}
	}()
}

func simulateWriteOperation(arc *ARC) {
	arc.Set(1, []byte("data1"))
	arc.Set(4, []byte("data4"))
	arc.Set(15, []byte("data15"))
	arc.Set(22, []byte("data22"))
	arc.Set(66, []byte("data66"))
	arc.Set(80, []byte("data80"))
}
