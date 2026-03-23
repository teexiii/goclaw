package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// mockAgentStoreForFiles wraps a minimal AgentStore for file endpoint tests.
type mockAgentStoreForFiles struct {
	agentStoreStub
	agent *store.AgentData
	files map[string]string // fileName → content
}

func (m *mockAgentStoreForFiles) GetByID(_ context.Context, id uuid.UUID) (*store.AgentData, error) {
	if m.agent != nil && m.agent.ID == id {
		return m.agent, nil
	}
	return nil, errMockNotFound
}

func (m *mockAgentStoreForFiles) GetByKey(_ context.Context, key string) (*store.AgentData, error) {
	if m.agent != nil && m.agent.AgentKey == key {
		return m.agent, nil
	}
	return nil, errMockNotFound
}

func (m *mockAgentStoreForFiles) GetAgentContextFiles(_ context.Context, _ uuid.UUID) ([]store.AgentContextFileData, error) {
	var result []store.AgentContextFileData
	for name, content := range m.files {
		result = append(result, store.AgentContextFileData{FileName: name, Content: content})
	}
	return result, nil
}

func (m *mockAgentStoreForFiles) SetAgentContextFile(_ context.Context, _ uuid.UUID, name, content string) error {
	m.files[name] = content
	return nil
}

func (m *mockAgentStoreForFiles) CanAccess(_ context.Context, _ uuid.UUID, _ string) (bool, string, error) {
	return true, "admin", nil
}

func newTestAgentFilesHandler() (*AgentsHandler, *mockAgentStoreForFiles) {
	ag := &store.AgentData{
		BaseModel:   store.BaseModel{ID: uuid.MustParse("00000000-0000-0000-0000-000000000010"), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		AgentKey:    "test-agent",
		DisplayName: "Test Agent",
		AgentType:   store.AgentTypeOpen,
	}
	ms := &mockAgentStoreForFiles{
		agent: ag,
		files: map[string]string{
			bootstrap.SoulFile:     "# Soul content",
			bootstrap.IdentityFile: "# Identity content",
		},
	}
	h := NewAgentsHandler(ms, "test-token", "/tmp/ws", nil, nil, nil)
	return h, ms
}

func agentFileReq(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("X-GoClaw-User-Id", "admin")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestAgentFilesListFiles(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("GET", "/v1/agents/test-agent/files", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	files, ok := result["files"].([]any)
	if !ok {
		t.Fatal("response missing 'files' field")
	}
	if len(files) != len(allowedAgentFiles) {
		t.Errorf("files count = %d, want %d", len(files), len(allowedAgentFiles))
	}
}

func TestAgentFilesGetFile(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("GET", "/v1/agents/test-agent/files/SOUL.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	file, ok := result["file"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'file' field")
	}
	if file["content"] != "# Soul content" {
		t.Errorf("content = %v, want '# Soul content'", file["content"])
	}
}

func TestAgentFilesGetFile_NotAllowed(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("GET", "/v1/agents/test-agent/files/SECRET.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAgentFilesGetFile_Missing(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("GET", "/v1/agents/test-agent/files/HEARTBEAT.md", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	file := result["file"].(map[string]any)
	if file["missing"] != true {
		t.Errorf("missing = %v, want true", file["missing"])
	}
}

func TestAgentFilesSetFile(t *testing.T) {
	setupTestCache(t, nil)
	h, ms := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("PUT", "/v1/agents/test-agent/files/SOUL.md", map[string]string{"content": "# New soul"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	if ms.files[bootstrap.SoulFile] != "# New soul" {
		t.Errorf("stored content = %q, want '# New soul'", ms.files[bootstrap.SoulFile])
	}
}

func TestAgentFilesSetFile_NotAllowed(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("PUT", "/v1/agents/test-agent/files/EVIL.md", map[string]string{"content": "hack"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAgentFilesSetFile_AgentNotFound(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := agentFileReq("PUT", "/v1/agents/nonexistent/files/SOUL.md", map[string]string{"content": "x"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAgentFilesNoAuth(t *testing.T) {
	setupTestCache(t, nil)
	h, _ := newTestAgentFilesHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/agents/test-agent/files", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
