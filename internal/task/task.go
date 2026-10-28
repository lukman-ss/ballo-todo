package task

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Task represents a simple task entity.
type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages an in-memory task store with thread-safe access.
type Store struct {
	mu     sync.RWMutex
	tasks  map[string]*Task
	nextID int64
}

// NewStore creates a new in-memory task store.
func NewStore() *Store {
	return &Store{
		tasks:  make(map[string]*Task),
		nextID: 1,
	}
}

// Create adds a new task. Returns error if title is invalid.
func (s *Store) Create(title string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title cannot be empty")
	}
	if len(title) > 200 {
		return nil, fmt.Errorf("title must be at most 200 characters")
	}

	id := fmt.Sprintf("%d", s.nextID)
	s.nextID++

	now := time.Now().UTC()
	t := &Task{
		ID:        id,
		Title:     title,
		Done:      false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.tasks[id] = t
	return t, nil
}

// GetByID retrieves a task by ID. Returns ErrNotFound if not found.
func (s *Store) GetByID(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Return a copy to prevent external mutation
	copy := *t
	return &copy, nil
}

// List returns all tasks sorted by ID. Returns a copy of all tasks.
func (s *Store) List() ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		copy := *t
		result = append(result, &copy)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// Update partially updates a task. Returns ErrNotFound if not found.
// Only updates fields that are provided (non-nil pointers).
func (s *Store) Update(id string, title *string, done *bool) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	if title != nil {
		t.Title = strings.TrimSpace(*title)
		if t.Title == "" {
			return nil, fmt.Errorf("title cannot be empty")
		}
		if len(t.Title) > 200 {
			return nil, fmt.Errorf("title must be at most 200 characters")
		}
	}
	if done != nil {
		t.Done = *done
	}

	t.UpdatedAt = time.Now().UTC()

	// Return a copy to prevent external mutation
	copy := *t
	return &copy, nil
}

// Delete removes a task by ID. Returns ErrNotFound if not found.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrNotFound
	}

	delete(s.tasks, id)
	return nil
}

// ErrNotFound indicates a task does not exist.
var ErrNotFound = fmt.Errorf("task not found")
