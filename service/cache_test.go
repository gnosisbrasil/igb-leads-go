package service

import (
	"testing"
	"time"
)

func TestCacheHitMissExpire(t *testing.T) {
	now := time.Now()
	c := NewCache(time.Minute)
	c.now = func() time.Time { return now }

	if _, ok := c.Get("a"); ok {
		t.Fatal("miss deveria retornar ok=false")
	}
	c.Set("a", 42)
	if v, ok := c.Get("a"); !ok || v != 42 {
		t.Fatalf("hit = %v,%v", v, ok)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expirado deveria retornar ok=false")
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := NewCache(time.Minute)
	c.Set("a", 1)
	c.Invalidate("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("após invalidate deveria ser miss")
	}
}
