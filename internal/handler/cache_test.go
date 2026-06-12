package handler

import (
	"fmt"
	"testing"
	"time"
)

func TestResponseCacheHitMissAndKeying(t *testing.T) {
	cache := newResponseCache(8, time.Minute)

	body := []byte(`{"origin":{"lat":52.4,"lon":4.7}}`)
	key := cache.Key("route", body)

	if _, ok := cache.Get(key); ok {
		t.Fatal("expected miss on empty cache")
	}

	cache.Set(key, []byte("response"))
	if got, ok := cache.Get(key); !ok || string(got) != "response" {
		t.Fatalf("expected hit, got %q ok=%v", got, ok)
	}

	// A modified request (e.g. a stop added to the transport file) must
	// produce a different key and therefore miss.
	changed := cache.Key("route", []byte(`{"origin":{"lat":52.4,"lon":4.7},"waypoints":[{"lat":51,"lon":5}]}`))
	if changed == key {
		t.Fatal("different bodies must produce different keys")
	}
	if _, ok := cache.Get(changed); ok {
		t.Fatal("expected miss for changed request")
	}

	// Same body, different endpoint → different key.
	if cache.Key("toll", body) == key {
		t.Fatal("endpoints must be keyed separately")
	}
}

func TestResponseCacheExpiry(t *testing.T) {
	cache := newResponseCache(8, 10*time.Millisecond)
	key := cache.Key("route", []byte("body"))
	cache.Set(key, []byte("response"))

	time.Sleep(20 * time.Millisecond)
	if _, ok := cache.Get(key); ok {
		t.Fatal("expected entry to expire")
	}
}

func TestResponseCacheEviction(t *testing.T) {
	cache := newResponseCache(3, time.Minute)
	for i := 0; i < 4; i++ {
		key := cache.Key("route", []byte(fmt.Sprintf("body-%d", i)))
		cache.Set(key, []byte("response"))
	}

	if _, ok := cache.Get(cache.Key("route", []byte("body-0"))); ok {
		t.Fatal("oldest entry should have been evicted")
	}
	if _, ok := cache.Get(cache.Key("route", []byte("body-3"))); !ok {
		t.Fatal("newest entry should be present")
	}
}
