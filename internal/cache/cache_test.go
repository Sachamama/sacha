package cache

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.Len() != 0 {
		t.Fatalf("new cache should be empty, got %d", c.Len())
	}
}

func TestSetAndGet(t *testing.T) {
	c := New()
	key := Key{AccountID: "123456789012", Region: "us-east-1", Service: "ec2"}

	items := []string{"i-abc", "i-def"}
	c.Set(key, items)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	slice, ok := got.([]string)
	if !ok {
		t.Fatal("expected []string")
	}
	if len(slice) != 2 {
		t.Fatalf("expected 2 items, got %d", len(slice))
	}
}

func TestGetMiss(t *testing.T) {
	c := New()
	key := Key{AccountID: "123456789012", Region: "us-east-1", Service: "ec2"}

	_, ok := c.Get(key)
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestDifferentKeys(t *testing.T) {
	c := New()

	key1 := Key{AccountID: "111", Region: "us-east-1", Service: "ec2"}
	key2 := Key{AccountID: "111", Region: "us-west-2", Service: "ec2"}
	key3 := Key{AccountID: "111", Region: "us-east-1", Service: "s3"}
	key4 := Key{AccountID: "222", Region: "us-east-1", Service: "ec2"}

	c.Set(key1, "data1")
	c.Set(key2, "data2")
	c.Set(key3, "data3")
	c.Set(key4, "data4")

	if c.Len() != 4 {
		t.Fatalf("expected 4 entries, got %d", c.Len())
	}

	got, ok := c.Get(key1)
	if !ok || got != "data1" {
		t.Fatalf("key1: expected data1, got %v", got)
	}

	got, ok = c.Get(key2)
	if !ok || got != "data2" {
		t.Fatalf("key2: expected data2, got %v", got)
	}

	got, ok = c.Get(key3)
	if !ok || got != "data3" {
		t.Fatalf("key3: expected data3, got %v", got)
	}

	got, ok = c.Get(key4)
	if !ok || got != "data4" {
		t.Fatalf("key4: expected data4, got %v", got)
	}
}

func TestDelete(t *testing.T) {
	c := New()
	key := Key{AccountID: "123", Region: "us-east-1", Service: "ec2"}

	c.Set(key, "data")
	c.Delete(key)

	_, ok := c.Get(key)
	if ok {
		t.Fatal("expected cache miss after delete")
	}
	if c.Len() != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", c.Len())
	}
}

func TestClear(t *testing.T) {
	c := New()

	c.Set(Key{AccountID: "1", Region: "r1", Service: "s1"}, "a")
	c.Set(Key{AccountID: "2", Region: "r2", Service: "s2"}, "b")
	c.Set(Key{AccountID: "3", Region: "r3", Service: "s3"}, "c")

	c.Clear()

	if c.Len() != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", c.Len())
	}
}

func TestOverwrite(t *testing.T) {
	c := New()
	key := Key{AccountID: "123", Region: "us-east-1", Service: "ec2"}

	c.Set(key, "old")
	c.Set(key, "new")

	got, ok := c.Get(key)
	if !ok || got != "new" {
		t.Fatalf("expected new, got %v", got)
	}
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", c.Len())
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New()
	key := Key{AccountID: "123", Region: "us-east-1", Service: "ec2"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set(key, val)
			c.Get(key)
		}(i)
	}
	wg.Wait()

	_, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit after concurrent writes")
	}
}
