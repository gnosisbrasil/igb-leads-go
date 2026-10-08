package service

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errDoom = errors.New("doom")

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

func TestDoSingleFlight(t *testing.T) {
	c := NewCache(time.Minute)
	var runs int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.Do("k", func() (any, error) {
				atomic.AddInt32(&runs, 1)
				time.Sleep(20 * time.Millisecond)
				return "v", nil
			})
			if err != nil || v != "v" {
				t.Errorf("Do = %v,%v", v, err)
			}
		}()
	}
	wg.Wait()
	if runs != 1 {
		t.Fatalf("fn executou %d vezes, want 1", runs)
	}
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("Get após Do = %v,%v", v, ok)
	}
}

func TestDoNaoCacheiaErro(t *testing.T) {
	c := NewCache(time.Minute)
	var runs int32
	fn := func() (any, error) {
		atomic.AddInt32(&runs, 1)
		return nil, errDoom
	}
	if _, err := c.Do("e", fn); err != errDoom {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.Do("e", fn); err != errDoom {
		t.Fatalf("err = %v", err)
	}
	if runs != 2 {
		t.Fatalf("fn executou %d vezes, want 2 (erro não cacheia)", runs)
	}
}
