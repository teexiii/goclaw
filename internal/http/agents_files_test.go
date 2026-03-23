package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ─── Mock agent store with context file support ──────────

// mockAgentFileStore extends mockAgentStore with in-memory context file storage.
type mockAgentFileStore struct {
	agentStoreStub
	mu           sync.Mutex
	agents       map[uuid.UUID]*store.AgentData
	byKey        map[string]*store.AgentData
	contextFiles map[uuid.UUID][]store.AgentContextFileData // agentID → files
	canAccess    map[string]bool                            // "agentID:userID" → allowed
}

func newMockAgentFileStore() *mockAgentFileStore {
	return &mockAgentFileStore{
		agents:       make(map[uuid.UUID]*store.AgentData),
		byKey:        make(map[string]*store.AgentData),
		contextFiles: make(map[uuid.UUID][]store.AgentContextFileData),
		canAccess:    make(map[string]bool),
	}
}

func (m *mockAgentFileStore) addAgent(ag *store.AgentData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[ag.ID] = ag
	m.byKey[ag.AgentKey] = ag
}

func (m *mockAgentFileStore) GetByID(_ context.Context, id uuid.UUID) (*store.AgentData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.agents[id]; ok {
		return a, nil
	}
	return nil, errNotFound
}

func (m *mockAgentFileStore) GetByKey(_ context.Context, key string) (*store.AgentData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.byKey[key]; ok {
		return a, nil
	}
	return nil, errNotFound
}

func (m *mockAgentFileStore) CanAccess(_ context.Context, agentID uuid.UUID, userID string) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := agentID.String() + ":" + userID
	if m.canAccess[key] {
		return true, "operator", nil
	}
	return false, "", nil
}

func (m *mockAgentFileStore) GetAgentContextFiles(_ context.Context, agentID uuid.UUID) ([]store.AgentContextFileData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.contextFiles[agentID], nil
}

func (m *mockAgentFileStore) SetAgentContextFile(_ context.Context, agentID uuid.UUID, fileName, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	files := m.contextFiles[agentID]
	for i, f := range files {
		if f.FileName == fileName {
			files[i].Content = content
			m.contextFiles[agentID] = files
			return nil
		}
	}
	m.contextFiles[agentID] = append(files, store.AgentContextFileData{
		AgentID:  agentID,
		FileName: fileName,
		Content:  content,
	})
	return nil
}

// ─── Test helpers ────────────────────────────────────────

var testAgentID = uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

func newTestAgentsHandler() (*AgentsHandler, *mockAgentFileStore) {
	as := newMockAgentFileStore()
	as.addAgent(&store.AgentData{
		BaseModel:   store.BaseModel{ID: testAgentID},
		AgentKey:    "test-agent",
		DisplayName: "Test Agent",
		AgentType:   store.AgentTypeOpen,
		OwnerID:     "owner-user",
	})
	// Owner has access to their own agent
	as.canAccess[testAgentID.String()+":owner-user"] = true
	h := NewAgentsHandler(as, "test-token", "/tmp/workspace", nil, nil, nil)
	return h, as
}

func newAgentsRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("X-GoClaw-User-Id", "owner-user")
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ─── Tests: GET /v1/agents/{id}/files ────────────────────

func TestListFiles(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Seed one file
	as.SetAgentContextFile(context.Background(), testAgentID, "SOUL.md", "You are a helpful agent.")

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	result := decodeJSON(t, w)
	files, ok := result["files"].([]any)
	if !ok {
		t.Fatal("response missing 'files' array")
	}
	if len(files) != len(allowedAgentFiles) {
		t.Errorf("files count = %d, want %d", len(files), len(allowedAgentFiles))
	}

	// Check SOUL.md is present and not missing
	for _, f := range files {
		fm := f.(map[string]any)
		if fm["name"] == "SOUL.md" {
			if fm["missing"] != false {
				t.Error("SOUL.md should not be missing")
			}
			if size, ok := fm["size"].(float64); !ok || size == 0 {
				t.Error("SOUL.md should have size > 0")
			}
			return
		}
	}
	t.Error("SOUL.md not found in file list")
}

func TestListFiles_AgentNotFound(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	fakeID := uuid.New()
	req := newAgentsRequest("GET", "/v1/agents/"+fakeID.String()+"/files", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ─── Tests: GET /v1/agents/{id}/files/{fileName} ────────

func TestGetFile(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	content := "# Soul\nYou are a helpful agent."
	as.SetAgentContextFile(context.Background(), testAgentID, "SOUL.md", content)

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	result := decodeJSON(t, w)
	file, ok := result["file"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'file' field")
	}
	if file["content"] != content {
		t.Errorf("content = %v, want %q", file["content"], content)
	}
	if file["missing"] != false {
		t.Error("file should not be missing")
	}
}

func TestGetFile_Missing(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	result := decodeJSON(t, w)
	file := result["file"].(map[string]any)
	if file["missing"] != true {
		t.Error("file should be missing")
	}
}

func TestGetFile_NotAllowed(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files/SECRET.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── Tests: PUT /v1/agents/{id}/files/{fileName} ────────

func TestSetFile(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]string{"content": "# Updated Soul\nNew personality."}
	req := newAgentsRequest("PUT", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	result := decodeJSON(t, w)
	file, ok := result["file"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'file' field")
	}
	if file["content"] != "# Updated Soul\nNew personality." {
		t.Errorf("content = %v, want updated content", file["content"])
	}

	// Verify stored in mock
	as.mu.Lock()
	files := as.contextFiles[testAgentID]
	as.mu.Unlock()
	if len(files) != 1 || files[0].Content != "# Updated Soul\nNew personality." {
		t.Errorf("file not saved in store; files = %+v", files)
	}
}

func TestSetFile_NotAllowed(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]string{"content": "hacked"}
	req := newAgentsRequest("PUT", "/v1/agents/"+testAgentID.String()+"/files/EVIL.md", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetFile_NonOwnerForbidden(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Grant access but not ownership
	as.mu.Lock()
	as.canAccess[testAgentID.String()+":other-user"] = true
	as.mu.Unlock()

	body := map[string]string{"content": "new content"}
	req := newAgentsRequest("PUT", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", body)
	req.Header.Set("X-GoClaw-User-Id", "other-user") // not the owner
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestSetFile_AgentNotFound(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	fakeID := uuid.New()
	body := map[string]string{"content": "content"}
	req := newAgentsRequest("PUT", "/v1/agents/"+fakeID.String()+"/files/SOUL.md", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetFile_InvalidJSON(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("PUT", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md",
		bytes.NewBufferString("not json"))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("X-GoClaw-User-Id", "owner-user")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetFile_Overwrite(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Write initial content
	as.SetAgentContextFile(context.Background(), testAgentID, "AGENTS.md", "old content")

	// Overwrite via API
	body := map[string]string{"content": "new content"}
	req := newAgentsRequest("PUT", "/v1/agents/"+testAgentID.String()+"/files/AGENTS.md", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify overwritten
	as.mu.Lock()
	files := as.contextFiles[testAgentID]
	as.mu.Unlock()
	for _, f := range files {
		if f.FileName == "AGENTS.md" {
			if f.Content != "new content" {
				t.Errorf("content = %q, want 'new content'", f.Content)
			}
			return
		}
	}
	t.Error("AGENTS.md not found in store after overwrite")
}

// ─── Tests: GET files with non-owner access ──────────────

func TestGetFile_NonOwnerWithAccess(t *testing.T) {
	setupTestCache(t, nil)
	h, as := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	as.SetAgentContextFile(context.Background(), testAgentID, "SOUL.md", "soul content")
	as.mu.Lock()
	as.canAccess[testAgentID.String()+":reader-user"] = true
	as.mu.Unlock()

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", nil)
	req.Header.Set("X-GoClaw-User-Id", "reader-user")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestGetFile_NonOwnerNoAccess(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newAgentsRequest("GET", "/v1/agents/"+testAgentID.String()+"/files/SOUL.md", nil)
	req.Header.Set("X-GoClaw-User-Id", "stranger-user")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
