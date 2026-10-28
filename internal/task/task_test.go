package task

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestStoreCreateAndGetByID(t *testing.T) {
	store := NewStore()

	task1, err := store.Create("Task one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task2, err := store.Create("Task two")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task1.ID == task2.ID {
		t.Fatal("expected unique IDs")
	}
	if task1.Title != "Task one" {
		t.Fatalf("expected Task one, got %q", task1.Title)
	}
	if task1.Done != false {
		t.Fatal("expected done=false")
	}
	if !task1.CreatedAt.Equal(task1.CreatedAt.UTC()) {
		t.Fatal("expected UTC CreatedAt")
	}

	fetched, err := store.GetByID(task1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched.ID != task1.ID || fetched.Title != task1.Title {
		t.Fatal("fetched task mismatch")
	}

	if _, err := store.GetByID("nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreValidation(t *testing.T) {
	store := NewStore()

	if _, err := store.Create(""); err == nil {
		t.Fatal("expected error for empty title")
	}
	if _, err := store.Create(strings.Repeat("x", 201)); err == nil {
		t.Fatal("expected error for title > 200 chars")
	}
}

func TestStoreListOrderAndCopy(t *testing.T) {
	store := NewStore()

	t1, _ := store.Create("first")
	t2, _ := store.Create("second")
	t3, _ := store.Create("third")

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != t1.ID || tasks[1].ID != t2.ID || tasks[2].ID != t3.ID {
		t.Fatalf("unexpected list order: %v", titles(tasks))
	}

	// Defensive copy: mutating returned task should not affect store
	tasks[0].Title = "mutated"
	tasks[0].Done = true

	again, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if again[0].Title != "first" || again[0].Done != false {
		t.Fatal("store mutated through returned slice")
	}
}

func TestStoreUpdatePartial(t *testing.T) {
	store := NewStore()
	created, err := store.Create("original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only done changes
	updated, err := store.Update(created.ID, nil, &boolTrue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "original" || updated.Done != true {
		t.Fatalf("partial update mismatch: %+v", updated)
	}

	// Only title changes
	updated2, err := store.Update(created.ID, &strNewTitle, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated2.Title != "new title" || updated2.Done != true {
		t.Fatalf("title update mismatch: %+v", updated2)
	}

	if _, err := store.Update("nonexistent", nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()
	created, err := store.Create("to delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := store.GetByID(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	if err := store.Delete(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for double delete, got %v", err)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			if _, err := store.Create("task"); err != nil {
				t.Errorf("create error: %v", err)
			}
		}(i)
		go func() {
			defer wg.Done()
			if _, err := store.List(); err != nil {
				t.Errorf("list error: %v", err)
			}
		}()
	}
	wg.Wait()

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 50 {
		t.Fatalf("expected 50 tasks, got %d", len(tasks))
	}
}

func titles(tasks []*Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Title
	}
	return out
}

var (
	strNewTitle = "new title"
	boolTrue    = true
)
