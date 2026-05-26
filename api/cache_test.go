package api

import "testing"

func TestNewLRUCacheUsesDefaultSize(t *testing.T) {
	cache := newLRUCache(0)
	if cache.maxSize != 500 {
		t.Fatalf("maxSize = %d, want 500", cache.maxSize)
	}
}

func TestLRUCacheDelete(t *testing.T) {
	cache := newLRUCache(2)
	cache.Store(&Email{ID: "msg-1"})
	cache.Delete("msg-1")
	if _, ok := cache.Load("msg-1"); ok {
		t.Fatal("expected deleted cache entry to be missing")
	}
}
