package storage

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentMapSingleClientStoreAndLoad(t *testing.T) {
	m := NewGenericConcurrentMap()
	m.Store("foo", GCMStringType{"bar"})
	m.Store("foo2", GCMIntegerType{2})
	val, ok := m.Load("foo")
	assert.Equal(t, ok, true)
	switch v := val.(type) {
	case GCMStringType:
		assert.Equal(t, v.GetValue(), "bar")
	default:
		assert.Fail(t, "Return value type must be string")
	}
	val, ok = m.Load("foo2")
	assert.Equal(t, ok, true)
	switch v := val.(type) {
	case GCMIntegerType:
		assert.Equal(t, v.GetValue(), 2)
	default:
		assert.Fail(t, "Return value type must be int")
	}
	_, ok = m.Load("foo3")
	assert.Equal(t, ok, false)
}

func TestConcurrentSingleClientMapDelete(t *testing.T) {
	m := NewGenericConcurrentMap()
	m.Store("foo", GCMStringType{"bar"})
	m.Store("foo2", GCMIntegerType{2})
	ok := m.Delete("foo")
	assert.Equal(t, ok, true)
	ok = m.Delete("foo2")
	assert.Equal(t, ok, true)
	ok = m.Delete("foo3")
	assert.Equal(t, ok, false)
}

func reader(t *testing.T, g *GenericConcurrentMap, c chan string, key string) {
	v, _ := g.Load(key)
	switch a := v.(type) {
	case GCMStringType:
		// Send value to channel
		c <- a.GetValue()
	default:
		assert.Fail(t, "Failed to read value with key:"+key)
	}
}

func writer(g *GenericConcurrentMap, c chan string, key string, value GCMStringType) {
	g.Store(key, value)
	c <- value.GetValue()
}

func TestConcurrentMapAccessMultipleClients(t *testing.T) {
	runtime.GOMAXPROCS(4)
	// Single writer, 2 readers
	// Ideas for this test are taken from https://golang.org/src/runtime/rwmutex_test.go
	m := NewGenericConcurrentMap()
	// Store initial value
	m.Store("foo", GCMStringType{"omg"})

	c := make(chan string, 1)
	done := make(chan string)

	// Enforce sequential access via channels
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, "omg")
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, "omg")
	go writer(m, done, "foo", GCMStringType{"lol"})
	<-done
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, "lol")
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, "lol")

	// Try concurrent reads without waiting, but waiting only on write
	go reader(t, m, c, "foo")
	go reader(t, m, c, "foo")
	go writer(m, done, "foo", GCMStringType{"lol"})
	go reader(t, m, c, "foo")
	<-done
	go reader(t, m, c, "foo")
	for i := 0; i < 4; i++ {
		val := <-c
		assert.Equal(t, val, "lol")
	}
}

func TestConcurrentMapWriteMultipleWriters(t *testing.T) {
	m := NewGenericConcurrentMap()
	done := make(chan string)
	c := make(chan string, 1)

	// We need this variable to hold the first value that is written. Because goroutines
	// can run concurrently, we don't know which write will succeed. By storing the return
	// value from write, we know which value to compare against
	var curr string

	// Two concurrent writers. Any may win first because we are only waiting for one
	go writer(m, done, "foo", GCMStringType{"lol"})
	go writer(m, done, "foo", GCMStringType{"lol2"})
	curr = <-done
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, curr)
	// If we now assert a reader, we may get lol or lol2, because we are not waiting on done.
	// We have no way of knowing which one without the wait
	curr = <-done
	go reader(t, m, c, "foo")
	assert.Equal(t, <-c, curr)
}

func TestConcurrentMapWriteAndDelete(t *testing.T) {
	m := NewGenericConcurrentMap()
	var wg sync.WaitGroup
	wg.Add(1)

	// If we schedule one after each other, it may fail.
	// There is no guarantee that write will finish first
	// Here we use a waitgroup to wait for counter to go to zero

	// Run write first
	go func() {
		m.Store("foo", GCMIntegerType{2})
		wg.Done()
	}()
	go func() {
		wg.Wait()
		// Waitgroup counter is now zero
		ok := m.Delete("foo")
		assert.Equal(t, ok, true)
	}()
	wg.Add(1)
	// Now run delete first
	go func() {
		wg.Wait()
		m.Store("foo", GCMIntegerType{2})
	}()
	go func() {
		ok := m.Delete("foo")
		assert.Equal(t, ok, false)
		wg.Done()
	}()
}