package blobcache

import (
	"sync"
	"testing"
)

func TestMemory_GetPutNotFound(t *testing.T) {
	c := NewMemory()

	if _, found, nf := c.Get("k"); found || nf {
		t.Error("unknown key should be neither found nor notFound")
	}

	c.Put("k", []byte("v"))
	if b, found, nf := c.Get("k"); !found || nf || string(b) != "v" {
		t.Errorf("expected hit 'v', got b=%q found=%v nf=%v", b, found, nf)
	}

	c.PutNotFound("missing")
	if _, found, nf := c.Get("missing"); found || !nf {
		t.Errorf("expected negative hit, got found=%v nf=%v", found, nf)
	}
}

func TestMemory_Concurrent(t *testing.T) {
	c := NewMemory()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			c.Put(key, []byte{byte(n)})
			_, _, _ = c.Get(key)
			c.PutNotFound("x" + key)
			_, _, _ = c.Get("x" + key)
		}(i)
	}
	wg.Wait()
}
