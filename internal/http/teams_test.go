package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

var errNotFound = errors.New("not found")

// ─── Mock stores ─────────────────────────────────────────

// mockTeamStore implements store.TeamStore with in-memory maps.
type mockTeamStore struct {
	mu      sync.Mutex
	teams   map[uuid.UUID]*store.TeamData
	members map[uuid.UUID][]store.TeamMemberData // teamID → members
	grants  map[uuid.UUID][]string               // teamID → userIDs with access
}

func newMockTeamStore() *mockTeamStore {
	return &mockTeamStore{
		teams:   make(map[uuid.UUID]*store.TeamData),
		members: make(map[uuid.UUID][]store.TeamMemberData),
		grants:  make(map[uuid.UUID][]string),
	}
}

func (m *mockTeamStore) CreateTeam(_ context.Context, team *store.TeamData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if team.ID == uuid.Nil {
		team.ID = uuid.New()
	}
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	m.teams[team.ID] = team
	return nil
}

func (m *mockTeamStore) GetTeam(_ context.Context, teamID uuid.UUID) (*store.TeamData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.teams[teamID]; ok {
		return t, nil
	}
	return nil, errNotFound
}

func (m *mockTeamStore) ListTeams(_ context.Context) ([]store.TeamData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.TeamData
	for _, t := range m.teams {
		result = append(result, *t)
	}
	return result, nil
}

func (m *mockTeamStore) AddMember(_ context.Context, teamID, agentID uuid.UUID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.members[teamID] = append(m.members[teamID], store.TeamMemberData{
		TeamID:   teamID,
		AgentID:  agentID,
		Role:     role,
		JoinedAt: time.Now(),
	})
	return nil
}

func (m *mockTeamStore) RemoveMember(_ context.Context, teamID, agentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	members := m.members[teamID]
	for i, mem := range members {
		if mem.AgentID == agentID {
			m.members[teamID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockTeamStore) ListMembers(_ context.Context, teamID uuid.UUID) ([]store.TeamMemberData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.members[teamID], nil
}

func (m *mockTeamStore) GetTeamForAgent(_ context.Context, agentID uuid.UUID) (*store.TeamData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.teams {
		if t.LeadAgentID == agentID {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockTeamStore) HasTeamAccess(_ context.Context, teamID uuid.UUID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.grants[teamID] {
		if u == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTeamStore) ListUserTeams(_ context.Context, userID string) ([]store.TeamData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.TeamData
	for teamID, users := range m.grants {
		for _, u := range users {
			if u == userID {
				if t, ok := m.teams[teamID]; ok {
					result = append(result, *t)
				}
			}
		}
	}
	return result, nil
}

// Stub out remaining TeamStore methods (unused by HTTP handler).
func (m *mockTeamStore) UpdateTeam(context.Context, uuid.UUID, map[string]any) error { return nil }
func (m *mockTeamStore) DeleteTeam(context.Context, uuid.UUID) error                 { return nil }
func (m *mockTeamStore) ListIdleMembers(context.Context, uuid.UUID) ([]store.TeamMemberData, error) {
	return nil, nil
}
func (m *mockTeamStore) KnownUserIDs(context.Context, uuid.UUID, int) ([]string, error) {
	return nil, nil
}
func (m *mockTeamStore) ListTaskScopes(context.Context, uuid.UUID) ([]store.ScopeEntry, error) {
	return nil, nil
}
func (m *mockTeamStore) CreateTask(context.Context, *store.TeamTaskData) error { return nil }
func (m *mockTeamStore) UpdateTask(context.Context, uuid.UUID, map[string]any) error {
	return nil
}
func (m *mockTeamStore) ListTasks(context.Context, uuid.UUID, string, string, string, string, string, int, int) ([]store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) GetTask(context.Context, uuid.UUID) (*store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) GetTasksByIDs(context.Context, []uuid.UUID) ([]store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) SearchTasks(context.Context, uuid.UUID, string, int, string) ([]store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) DeleteTask(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockTeamStore) DeleteTasks(context.Context, []uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockTeamStore) ClaimTask(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (m *mockTeamStore) AssignTask(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (m *mockTeamStore) CompleteTask(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockTeamStore) CancelTask(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (m *mockTeamStore) FailTask(context.Context, uuid.UUID, uuid.UUID, string) error   { return nil }
func (m *mockTeamStore) FailPendingTask(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockTeamStore) ReviewTask(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockTeamStore) ApproveTask(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockTeamStore) RejectTask(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (m *mockTeamStore) AddTaskComment(context.Context, *store.TeamTaskCommentData) error {
	return nil
}
func (m *mockTeamStore) ListTaskComments(context.Context, uuid.UUID) ([]store.TeamTaskCommentData, error) {
	return nil, nil
}
func (m *mockTeamStore) ListRecentTaskComments(context.Context, uuid.UUID, int) ([]store.TeamTaskCommentData, error) {
	return nil, nil
}
func (m *mockTeamStore) RecordTaskEvent(context.Context, *store.TeamTaskEventData) error { return nil }
func (m *mockTeamStore) ListTaskEvents(context.Context, uuid.UUID) ([]store.TeamTaskEventData, error) {
	return nil, nil
}
func (m *mockTeamStore) ListTeamEvents(context.Context, uuid.UUID, int, int) ([]store.TeamTaskEventData, error) {
	return nil, nil
}
func (m *mockTeamStore) AttachFileToTask(context.Context, *store.TeamTaskAttachmentData) error {
	return nil
}
func (m *mockTeamStore) GetAttachment(context.Context, uuid.UUID) (*store.TeamTaskAttachmentData, error) {
	return nil, nil
}
func (m *mockTeamStore) ListTaskAttachments(context.Context, uuid.UUID) ([]store.TeamTaskAttachmentData, error) {
	return nil, nil
}
func (m *mockTeamStore) DetachFileFromTask(context.Context, uuid.UUID, string) error { return nil }
func (m *mockTeamStore) SetTaskFollowup(context.Context, uuid.UUID, uuid.UUID, time.Time, int, string, string, string) error {
	return nil
}
func (m *mockTeamStore) ClearTaskFollowup(context.Context, uuid.UUID) error { return nil }
func (m *mockTeamStore) ListAllFollowupDueTasks(context.Context) ([]store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) IncrementFollowupCount(context.Context, uuid.UUID, *time.Time) error {
	return nil
}
func (m *mockTeamStore) ClearFollowupByScope(context.Context, string, string) (int, error) {
	return 0, nil
}
func (m *mockTeamStore) SetFollowupForActiveTasks(context.Context, uuid.UUID, string, string, time.Time, int, string) (int, error) {
	return 0, nil
}
func (m *mockTeamStore) HasActiveMemberTasks(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockTeamStore) UpdateTaskProgress(context.Context, uuid.UUID, uuid.UUID, int, string) error {
	return nil
}
func (m *mockTeamStore) RenewTaskLock(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockTeamStore) RecoverAllStaleTasks(context.Context) ([]store.RecoveredTaskInfo, error) {
	return nil, nil
}
func (m *mockTeamStore) ForceRecoverAllTasks(context.Context) ([]store.RecoveredTaskInfo, error) {
	return nil, nil
}
func (m *mockTeamStore) ListRecoverableTasks(context.Context, uuid.UUID) ([]store.TeamTaskData, error) {
	return nil, nil
}
func (m *mockTeamStore) MarkAllStaleTasks(_ context.Context, _ time.Time) ([]store.RecoveredTaskInfo, error) {
	return nil, nil
}
func (m *mockTeamStore) ResetTaskStatus(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockTeamStore) GrantTeamAccess(context.Context, uuid.UUID, string, string, string) error {
	return nil
}
func (m *mockTeamStore) RevokeTeamAccess(context.Context, uuid.UUID, string) error { return nil }
func (m *mockTeamStore) ListTeamGrants(context.Context, uuid.UUID) ([]store.TeamUserGrant, error) {
	return nil, nil
}

// agentStoreStub provides no-op implementations for every AgentStore method.
type agentStoreStub struct{}

func (agentStoreStub) Create(context.Context, *store.AgentData) error                                          { return nil }
func (agentStoreStub) GetByKey(context.Context, string) (*store.AgentData, error)                               { return nil, errNotFound }
func (agentStoreStub) GetByID(context.Context, uuid.UUID) (*store.AgentData, error)                             { return nil, errNotFound }
func (agentStoreStub) GetByKeys(context.Context, []string) ([]store.AgentData, error)                           { return nil, nil }
func (agentStoreStub) GetByIDs(context.Context, []uuid.UUID) ([]store.AgentData, error)                         { return nil, nil }
func (agentStoreStub) Update(context.Context, uuid.UUID, map[string]any) error                                  { return nil }
func (agentStoreStub) Delete(context.Context, uuid.UUID) error                                                  { return nil }
func (agentStoreStub) List(context.Context, string) ([]store.AgentData, error)                                  { return nil, nil }
func (agentStoreStub) GetDefault(context.Context) (*store.AgentData, error)                                     { return nil, nil }
func (agentStoreStub) ShareAgent(context.Context, uuid.UUID, string, string, string) error                      { return nil }
func (agentStoreStub) RevokeShare(context.Context, uuid.UUID, string) error                                     { return nil }
func (agentStoreStub) ListShares(context.Context, uuid.UUID) ([]store.AgentShareData, error)                    { return nil, nil }
func (agentStoreStub) CanAccess(context.Context, uuid.UUID, string) (bool, string, error)                       { return false, "", nil }
func (agentStoreStub) ListAccessible(context.Context, string) ([]store.AgentData, error)                        { return nil, nil }
func (agentStoreStub) GetAgentContextFiles(context.Context, uuid.UUID) ([]store.AgentContextFileData, error)    { return nil, nil }
func (agentStoreStub) SetAgentContextFile(context.Context, uuid.UUID, string, string) error                     { return nil }
func (agentStoreStub) GetUserContextFiles(context.Context, uuid.UUID, string) ([]store.UserContextFileData, error) { return nil, nil }
func (agentStoreStub) SetUserContextFile(context.Context, uuid.UUID, string, string, string) error              { return nil }
func (agentStoreStub) DeleteUserContextFile(context.Context, uuid.UUID, string, string) error                   { return nil }
func (agentStoreStub) GetUserOverride(context.Context, uuid.UUID, string) (*store.UserAgentOverrideData, error) { return nil, nil }
func (agentStoreStub) SetUserOverride(context.Context, *store.UserAgentOverrideData) error                      { return nil }
func (agentStoreStub) GetOrCreateUserProfile(context.Context, uuid.UUID, string, string, string) (bool, string, error) { return false, "", nil }
func (agentStoreStub) EnsureUserProfile(context.Context, uuid.UUID, string) error                               { return nil }
func (agentStoreStub) ListUserInstances(context.Context, uuid.UUID) ([]store.UserInstanceData, error)           { return nil, nil }
func (agentStoreStub) UpdateUserProfileMetadata(context.Context, uuid.UUID, string, map[string]string) error    { return nil }

// mockAgentStore overrides GetByID and GetByKey with in-memory maps.
type mockAgentStore struct {
	agentStoreStub
	mu     sync.Mutex
	agents map[uuid.UUID]*store.AgentData
	byKey  map[string]*store.AgentData
}

func newMockAgentStore() *mockAgentStore {
	return &mockAgentStore{
		agents: make(map[uuid.UUID]*store.AgentData),
		byKey:  make(map[string]*store.AgentData),
	}
}

func (m *mockAgentStore) addAgent(ag *store.AgentData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[ag.ID] = ag
	m.byKey[ag.AgentKey] = ag
}

func (m *mockAgentStore) GetByID(_ context.Context, id uuid.UUID) (*store.AgentData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.agents[id]; ok {
		return a, nil
	}
	return nil, errNotFound
}

func (m *mockAgentStore) GetByKey(_ context.Context, key string) (*store.AgentData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.byKey[key]; ok {
		return a, nil
	}
	return nil, errNotFound
}

// mockLinkStore implements the AgentLinkStore methods used by TeamsHandler.
type mockLinkStore struct {
	mu    sync.Mutex
	links []store.AgentLinkData
}

func newMockLinkStore() *mockLinkStore { return &mockLinkStore{} }

func (m *mockLinkStore) CreateLink(_ context.Context, link *store.AgentLinkData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	link.ID = uuid.New()
	m.links = append(m.links, *link)
	return nil
}

func (m *mockLinkStore) DeleteTeamLinksForAgent(_ context.Context, teamID, agentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var kept []store.AgentLinkData
	for _, l := range m.links {
		if l.TeamID != nil && *l.TeamID == teamID && (l.SourceAgentID == agentID || l.TargetAgentID == agentID) {
			continue
		}
		kept = append(kept, l)
	}
	m.links = kept
	return nil
}

// Stub remaining AgentLinkStore methods.
func (m *mockLinkStore) DeleteLink(context.Context, uuid.UUID) error                 { return nil }
func (m *mockLinkStore) UpdateLink(context.Context, uuid.UUID, map[string]any) error { return nil }
func (m *mockLinkStore) GetLink(context.Context, uuid.UUID) (*store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) ListLinksFrom(context.Context, uuid.UUID) ([]store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) ListLinksTo(context.Context, uuid.UUID) ([]store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) CanDelegate(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockLinkStore) GetLinkBetween(context.Context, uuid.UUID, uuid.UUID) (*store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) DelegateTargets(context.Context, uuid.UUID) ([]store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) SearchDelegateTargets(context.Context, uuid.UUID, string, int) ([]store.AgentLinkData, error) {
	return nil, nil
}
func (m *mockLinkStore) SearchDelegateTargetsByEmbedding(context.Context, uuid.UUID, []float32, int) ([]store.AgentLinkData, error) {
	return nil, nil
}

// ─── Test helpers ────────────────────────────────────────

var (
	leadID     = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	memberID   = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	reviewerID = uuid.MustParse("00000000-0000-0000-0000-000000000003")
)

func seedAgents(as *mockAgentStore) {
	as.addAgent(&store.AgentData{
		BaseModel:   store.BaseModel{ID: leadID},
		AgentKey:    "lead-agent",
		DisplayName: "Lead Agent",
		AgentType:   store.AgentTypeOpen,
	})
	as.addAgent(&store.AgentData{
		BaseModel:   store.BaseModel{ID: memberID},
		AgentKey:    "member-agent",
		DisplayName: "Member Agent",
		AgentType:   store.AgentTypeOpen,
	})
	as.addAgent(&store.AgentData{
		BaseModel:   store.BaseModel{ID: reviewerID},
		AgentKey:    "reviewer-agent",
		DisplayName: "Reviewer Agent",
		AgentType:   store.AgentTypeOpen,
	})
}

func newTestTeamsHandler() (*TeamsHandler, *mockTeamStore, *mockAgentStore, *mockLinkStore) {
	ts := newMockTeamStore()
	as := newMockAgentStore()
	ls := newMockLinkStore()
	seedAgents(as)
	h := NewTeamsHandler(ts, as, ls, "test-token", nil)
	return h, ts, as, ls
}

func newTeamsRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("X-GoClaw-User-Id", "admin-user")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, w.Body.String())
	}
	return result
}

// ─── Tests ───────────────────────────────────────────────

func TestTeamsCreate(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, ls := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("POST", "/v1/teams", teamCreateRequest{
		Name:    "Translation Team",
		Lead:    "lead-agent",
		Members: []string{"member-agent", "reviewer-agent"},
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	result := decodeJSON(t, w)
	teamData, ok := result["team"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'team' field")
	}
	if teamData["name"] != "Translation Team" {
		t.Errorf("team name = %v, want 'Translation Team'", teamData["name"])
	}

	// Verify team was stored
	ts.mu.Lock()
	if len(ts.teams) != 1 {
		t.Errorf("teams count = %d, want 1", len(ts.teams))
	}
	ts.mu.Unlock()

	// Verify members added (lead + 2 members = 3)
	var teamID uuid.UUID
	ts.mu.Lock()
	for id := range ts.teams {
		teamID = id
	}
	memberCount := len(ts.members[teamID])
	ts.mu.Unlock()
	if memberCount != 3 {
		t.Errorf("member count = %d, want 3", memberCount)
	}

	// Verify links created (lead → member, lead → reviewer = 2)
	ls.mu.Lock()
	linkCount := len(ls.links)
	ls.mu.Unlock()
	if linkCount != 2 {
		t.Errorf("link count = %d, want 2", linkCount)
	}
}

func TestTeamsCreate_MissingName(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("POST", "/v1/teams", teamCreateRequest{
		Lead: "lead-agent",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsCreate_MissingLead(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("POST", "/v1/teams", teamCreateRequest{
		Name: "My Team",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsCreate_UnknownAgent(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("POST", "/v1/teams", teamCreateRequest{
		Name: "My Team",
		Lead: "nonexistent-agent",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsCreate_DuplicateLeadership(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Create first team
	ts.mu.Lock()
	existingID := uuid.New()
	ts.teams[existingID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: existingID},
		Name:        "Existing Team",
		LeadAgentID: leadID,
		Status:      store.TeamStatusActive,
	}
	ts.mu.Unlock()

	// Try to create second team with same lead
	req := newTeamsRequest("POST", "/v1/teams", teamCreateRequest{
		Name: "Second Team",
		Lead: "lead-agent",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestTeamsCreate_NoAuth(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/teams", bytes.NewBufferString(`{"name":"Team","lead":"lead-agent"}`))
	req.Header.Set("Content-Type", "application/json")
	// no Authorization header
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestTeamsList(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Seed a team
	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
		Status:      store.TeamStatusActive,
	}
	ts.mu.Unlock()

	req := newTeamsRequest("GET", "/v1/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	result := decodeJSON(t, w)
	count, _ := result["count"].(float64)
	if count != 1 {
		t.Errorf("count = %v, want 1", count)
	}
}

func TestTeamsGet(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
		Status:      store.TeamStatusActive,
	}
	ts.members[teamID] = []store.TeamMemberData{
		{TeamID: teamID, AgentID: leadID, Role: store.TeamRoleLead},
		{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember},
	}
	ts.mu.Unlock()

	req := newTeamsRequest("GET", "/v1/teams/"+teamID.String(), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	result := decodeJSON(t, w)
	members, ok := result["members"].([]any)
	if !ok {
		t.Fatal("response missing 'members' field")
	}
	if len(members) != 2 {
		t.Errorf("members count = %d, want 2", len(members))
	}
}

func TestTeamsGet_NotFound(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("GET", "/v1/teams/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestTeamsGet_InvalidID(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := newTeamsRequest("GET", "/v1/teams/not-a-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsAddMember(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, ls := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
		Status:      store.TeamStatusActive,
	}
	ts.members[teamID] = []store.TeamMemberData{
		{TeamID: teamID, AgentID: leadID, Role: store.TeamRoleLead},
	}
	ts.mu.Unlock()

	req := newTeamsRequest("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{
		Agent: "member-agent",
		Role:  "member",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify member was added
	ts.mu.Lock()
	memberCount := len(ts.members[teamID])
	ts.mu.Unlock()
	if memberCount != 2 {
		t.Errorf("member count = %d, want 2", memberCount)
	}

	// Verify link was created
	ls.mu.Lock()
	linkCount := len(ls.links)
	ls.mu.Unlock()
	if linkCount != 1 {
		t.Errorf("link count = %d, want 1", linkCount)
	}
}

func TestTeamsAddMember_PreventAddingLead(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.mu.Unlock()

	req := newTeamsRequest("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{
		Agent: "lead-agent",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsAddMember_InvalidRole(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.mu.Unlock()

	req := newTeamsRequest("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{
		Agent: "member-agent",
		Role:  "admin",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTeamsAddMember_ReviewerRole(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.mu.Unlock()

	req := newTeamsRequest("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{
		Agent: "reviewer-agent",
		Role:  "reviewer",
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	ts.mu.Lock()
	members := ts.members[teamID]
	ts.mu.Unlock()
	if len(members) != 1 || members[0].Role != store.TeamRoleReviewer {
		t.Errorf("expected reviewer role, got %v", members)
	}
}

func TestTeamsRemoveMember(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.members[teamID] = []store.TeamMemberData{
		{TeamID: teamID, AgentID: leadID, Role: store.TeamRoleLead},
		{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember},
	}
	ts.mu.Unlock()

	req := newTeamsRequest("DELETE", "/v1/teams/"+teamID.String()+"/members/"+memberID.String(), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	ts.mu.Lock()
	memberCount := len(ts.members[teamID])
	ts.mu.Unlock()
	if memberCount != 1 {
		t.Errorf("member count = %d, want 1 (only lead)", memberCount)
	}
}

func TestTeamsRemoveMember_ByAgentKey(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.members[teamID] = []store.TeamMemberData{
		{TeamID: teamID, AgentID: leadID, Role: store.TeamRoleLead},
		{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember},
	}
	ts.mu.Unlock()

	// Use agent key instead of UUID
	req := newTeamsRequest("DELETE", "/v1/teams/"+teamID.String()+"/members/member-agent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestTeamsRemoveMember_CannotRemoveLead(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	ts.mu.Lock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel:   store.BaseModel{ID: teamID},
		Name:        "Test Team",
		LeadAgentID: leadID,
	}
	ts.mu.Unlock()

	req := newTeamsRequest("DELETE", "/v1/teams/"+teamID.String()+"/members/"+leadID.String(), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
