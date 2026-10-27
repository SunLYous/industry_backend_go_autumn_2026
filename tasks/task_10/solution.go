package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type TaskRepo interface {
	Create(title string) (Task, error)
	Get(id string) (Task, bool)
	List(done bool) []Task
	SetDone(id string, done bool) (Task, error)
}
type Clock interface{ Now() time.Time }

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidTitle = errors.New("invalid title")
)

type inMemoryTaskRepo struct {
	mu    sync.RWMutex
	clock Clock
	seq   uint64
	tasks map[string]Task
}

func NewInMemoryTaskRepo(clock Clock) TaskRepo {
	return &inMemoryTaskRepo{clock: clock, tasks: make(map[string]Task)}
}
func (r *inMemoryTaskRepo) Create(title string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrInvalidTitle
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	task := Task{
		ID:        strconv.FormatUint(r.seq, 10),
		Title:     title,
		Done:      false,
		UpdatedAt: r.clock.Now(),
	}
	r.tasks[task.ID] = task

	return task, nil
}
func (r *inMemoryTaskRepo) Get(id string) (Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[id]

	return task, ok
}
func (r *inMemoryTaskRepo) List(done bool) []Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Task, 0)
	for _, task := range r.tasks {
		if task.Done == done {
			out = append(out, task)
		}
	}

	return out
}
func (r *inMemoryTaskRepo) SetDone(id string, done bool) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	if task.Done == done {
		return task, nil
	}

	task.Done = done
	task.UpdatedAt = r.clock.Now()
	r.tasks[id] = task

	return task, nil
}

type httpHandler struct {
	repo TaskRepo
	mux  *http.ServeMux
}

func NewHTTPHandler(repo TaskRepo) http.Handler {
	h := &httpHandler{repo: repo, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /tasks", h.handleCreate)
	h.mux.HandleFunc("GET /tasks", h.handleList)
	h.mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		h.handleGet(w, r.PathValue("id"))
	})
	h.mux.HandleFunc("PATCH /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		h.handlePatch(w, r, r.PathValue("id"))
	})
	return h
}
func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
func (h *httpHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title *string `json:"title"`
	}
	if err := decodeStrictJSON(r.Body, &body); err != nil || body.Title == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	task, err := h.repo.Create(*body.Title)
	switch {
	case errors.Is(err, ErrInvalidTitle):
		http.Error(w, "invalid title", http.StatusBadRequest)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		writeJSON(w, http.StatusCreated, task)
	}
}
func (h *httpHandler) handleGet(w http.ResponseWriter, id string) {
	task, ok := h.repo.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, task)
}
func (h *httpHandler) handleList(w http.ResponseWriter, r *http.Request) {
	done := false
	if vals, present := r.URL.Query()["done"]; present {
		if len(vals) != 1 || (vals[0] != "true" && vals[0] != "false") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		done = vals[0] == "true"
	}

	tasks := h.repo.List(done)
	if tasks == nil {
		tasks = []Task{}
	}
	slices.SortFunc(tasks, func(a, b Task) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	writeJSON(w, http.StatusOK, tasks)
}
func (h *httpHandler) handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Done *bool `json:"done"`
	}
	if err := decodeStrictJSON(r.Body, &body); err != nil || body.Done == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	task, err := h.repo.SetDone(id, *body.Done)
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		writeJSON(w, http.StatusOK, task)
	}
}
func decodeStrictJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON value")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
