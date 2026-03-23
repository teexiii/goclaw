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

var errMockNotFound = errors.New("not found")

// ─── Mock stores ─────────────────────────────────────────

type mockTeamStore struct {
	mu      sync.Mutex
	teams   map[uuid.UUID]*store.TeamData
	members map[uuid.UUID][]store.TeamMemberData
	grants  map[uuid.UUID][]string
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
	return nil, errMockNotFound
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
		TeamID: teamID, AgentID: agentID, Role: role, JoinedAt: time.Now(),
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

// Stub remaining TeamStore methods.
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
func (m *mockTeamStore) CreateTask(context.Context, *store.TeamTaskData) error          { return nil }
func (m *mockTeamStore) UpdateTask(context.Context, uuid.UUID, map[string]any) error    { return nil }
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

// ─── Agent store stub ────────────────────────────────────

type agentStoreStub struct{}

func (agentStoreStub) Create(context.Context, *store.AgentData) error                    { return nil }
func (agentStoreStub) GetByKey(context.Context, string) (*store.AgentData, error)         { return nil, errMockNotFound }
func (agentStoreStub) GetByID(context.Context, uuid.UUID) (*store.AgentData, error)       { return nil, errMockNotFound }
func (agentStoreStub) GetByKeys(context.Context, []string) ([]store.AgentData, error)     { return nil, nil }
func (agentStoreStub) GetByIDs(context.Context, []uuid.UUID) ([]store.AgentData, error)   { return nil, nil }
func (agentStoreStub) Update(context.Context, uuid.UUID, map[string]any) error            { return nil }
func (agentStoreStub) Delete(context.Context, uuid.UUID) error                            { return nil }
func (agentStoreStub) List(context.Context, string) ([]store.AgentData, error)            { return nil, nil }
func (agentStoreStub) GetDefault(context.Context) (*store.AgentData, error)               { return nil, nil }
func (agentStoreStub) ShareAgent(context.Context, uuid.UUID, string, string, string) error { return nil }
func (agentStoreStub) RevokeShare(context.Context, uuid.UUID, string) error               { return nil }
func (agentStoreStub) ListShares(context.Context, uuid.UUID) ([]store.AgentShareData, error) { return nil, nil }
func (agentStoreStub) CanAccess(context.Context, uuid.UUID, string) (bool, string, error) { return false, "", nil }
func (agentStoreStub) ListAccessible(context.Context, string) ([]store.AgentData, error)  { return nil, nil }
func (agentStoreStub) GetAgentContextFiles(context.Context, uuid.UUID) ([]store.AgentContextFileData, error) { return nil, nil }
func (agentStoreStub) SetAgentContextFile(context.Context, uuid.UUID, string, string) error { return nil }
func (agentStoreStub) GetUserContextFiles(context.Context, uuid.UUID, string) ([]store.UserContextFileData, error) { return nil, nil }
func (agentStoreStub) SetUserContextFile(context.Context, uuid.UUID, string, string, string) error { return nil }
func (agentStoreStub) DeleteUserContextFile(context.Context, uuid.UUID, string, string) error { return nil }
func (agentStoreStub) GetUserOverride(context.Context, uuid.UUID, string) (*store.UserAgentOverrideData, error) { return nil, nil }
func (agentStoreStub) SetUserOverride(context.Context, *store.UserAgentOverrideData) error { return nil }
func (agentStoreStub) GetOrCreateUserProfile(context.Context, uuid.UUID, string, string, string) (bool, string, error) { return false, "", nil }
func (agentStoreStub) EnsureUserProfile(context.Context, uuid.UUID, string) error         { return nil }
func (agentStoreStub) ListUserInstances(context.Context, uuid.UUID) ([]store.UserInstanceData, error) { return nil, nil }
func (agentStoreStub) UpdateUserProfileMetadata(context.Context, uuid.UUID, string, map[string]string) error { return nil }

type mockAgentStore struct {
	agentStoreStub
	mu     sync.Mutex
	agents map[uuid.UUID]*store.AgentData
	byKey  map[string]*store.AgentData
}

func newMockAgentStore() *mockAgentStore {
	return &mockAgentStore{agents: make(map[uuid.UUID]*store.AgentData), byKey: make(map[string]*store.AgentData)}
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
	return nil, errMockNotFound
}

func (m *mockAgentStore) GetByKey(_ context.Context, key string) (*store.AgentData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.byKey[key]; ok {
		return a, nil
	}
	return nil, errMockNotFound
}

// ─── Link store mock ─────────────────────────────────────

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

func (m *mockLinkStore) DeleteLink(context.Context, uuid.UUID) error                 { return nil }
func (m *mockLinkStore) UpdateLink(context.Context, uuid.UUID, map[string]any) error { return nil }
func (m *mockLinkStore) GetLink(context.Context, uuid.UUID) (*store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) ListLinksFrom(context.Context, uuid.UUID) ([]store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) ListLinksTo(context.Context, uuid.UUID) ([]store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) CanDelegate(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil }
func (m *mockLinkStore) GetLinkBetween(context.Context, uuid.UUID, uuid.UUID) (*store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) DelegateTargets(context.Context, uuid.UUID) ([]store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) SearchDelegateTargets(context.Context, uuid.UUID, string, int) ([]store.AgentLinkData, error) { return nil, nil }
func (m *mockLinkStore) SearchDelegateTargetsByEmbedding(context.Context, uuid.UUID, []float32, int) ([]store.AgentLinkData, error) { return nil, nil }

// ─── Test helpers ────────────────────────────────────────

var (
	leadID     = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	memberID   = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	reviewerID = uuid.MustParse("00000000-0000-0000-0000-000000000003")
)

func seedAgents(as *mockAgentStore) {
	as.addAgent(&store.AgentData{BaseModel: store.BaseModel{ID: leadID}, AgentKey: "lead-agent", DisplayName: "Lead"})
	as.addAgent(&store.AgentData{BaseModel: store.BaseModel{ID: memberID}, AgentKey: "member-agent", DisplayName: "Member"})
	as.addAgent(&store.AgentData{BaseModel: store.BaseModel{ID: reviewerID}, AgentKey: "reviewer-agent", DisplayName: "Reviewer"})
}

func newTestTeamsHandler() (*TeamsHandler, *mockTeamStore, *mockAgentStore, *mockLinkStore) {
	ts := newMockTeamStore()
	as := newMockAgentStore()
	ls := newMockLinkStore()
	seedAgents(as)
	return NewTeamsHandler(ts, as, ls, "test-token", nil), ts, as, ls
}

func teamReq(method, path string, body any) *http.Request {
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

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var r map[string]any
	if err := json.NewDecoder(w.Body).Decode(&r); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, w.Body.String())
	}
	return r
}

func seedTeam(ts *mockTeamStore, teamID uuid.UUID) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.teams[teamID] = &store.TeamData{
		BaseModel: store.BaseModel{ID: teamID}, Name: "Test Team", LeadAgentID: leadID, Status: store.TeamStatusActive,
	}
	ts.members[teamID] = []store.TeamMemberData{
		{TeamID: teamID, AgentID: leadID, Role: store.TeamRoleLead},
	}
}

// ─── Tests: Create ───────────────────────────────────────

func TestTeamsCreate(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, ls := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{
		Name: "Translation Team", Lead: "lead-agent", Members: []string{"member-agent", "reviewer-agent"},
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	ts.mu.Lock()
	teamCount := len(ts.teams)
	var teamID uuid.UUID
	for id := range ts.teams {
		teamID = id
	}
	memberCount := len(ts.members[teamID])
	ts.mu.Unlock()

	if teamCount != 1 {
		t.Errorf("teams = %d, want 1", teamCount)
	}
	if memberCount != 3 { // lead + 2 members
		t.Errorf("members = %d, want 3", memberCount)
	}
	ls.mu.Lock()
	if len(ls.links) != 2 { // lead→member, lead→reviewer
		t.Errorf("links = %d, want 2", len(ls.links))
	}
	ls.mu.Unlock()
}

func TestTeamsCreate_DeduplicatesLeadFromMembers(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{
		Name: "Team", Lead: "lead-agent", Members: []string{"lead-agent", "member-agent", "member-agent"},
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}

	ts.mu.Lock()
	var teamID uuid.UUID
	for id := range ts.teams {
		teamID = id
	}
	memberCount := len(ts.members[teamID])
	ts.mu.Unlock()
	if memberCount != 2 { // lead + member-agent (deduped)
		t.Errorf("members = %d, want 2", memberCount)
	}
}

func TestTeamsCreate_MissingName(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{Lead: "lead-agent"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTeamsCreate_MissingLead(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{Name: "Team"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTeamsCreate_UnknownAgent(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{Name: "Team", Lead: "ghost"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTeamsCreate_DuplicateLeadership(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	seedTeam(ts, uuid.New())

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams", teamCreateRequest{Name: "Second", Lead: "lead-agent"}))
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestTeamsCreate_NoAuth(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/teams", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// ─── Tests: List & Get ───────────────────────────────────

func TestTeamsList(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	seedTeam(ts, uuid.New())

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("GET", "/v1/teams", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	r := decode(t, w)
	if r["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", r["count"])
	}
}

func TestTeamsGet(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("GET", "/v1/teams/"+teamID.String(), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	r := decode(t, w)
	if _, ok := r["members"]; !ok {
		t.Error("response missing 'members'")
	}
}

func TestTeamsGet_NotFound(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("GET", "/v1/teams/"+uuid.New().String(), nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestTeamsGet_InvalidID(t *testing.T) {
	setupTestCache(t, nil)
	h, _, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("GET", "/v1/teams/not-a-uuid", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ─── Tests: Add Member ───────────────────────────────────

func TestTeamsAddMember(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, ls := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{Agent: "member-agent"}))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	ts.mu.Lock()
	if len(ts.members[teamID]) != 2 {
		t.Errorf("members = %d, want 2", len(ts.members[teamID]))
	}
	ts.mu.Unlock()

	ls.mu.Lock()
	if len(ls.links) != 1 {
		t.Errorf("links = %d, want 1", len(ls.links))
	}
	ls.mu.Unlock()
}

func TestTeamsAddMember_DuplicateReturns409(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)
	ts.mu.Lock()
	ts.members[teamID] = append(ts.members[teamID], store.TeamMemberData{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember})
	ts.mu.Unlock()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{Agent: "member-agent"}))
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestTeamsAddMember_PreventLead(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{Agent: "lead-agent"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTeamsAddMember_InvalidRole(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{Agent: "member-agent", Role: "admin"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTeamsAddMember_ReviewerRole(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("POST", "/v1/teams/"+teamID.String()+"/members", addMemberRequest{Agent: "reviewer-agent", Role: "reviewer"}))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// ─── Tests: Remove Member ────────────────────────────────

func TestTeamsRemoveMember(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)
	ts.mu.Lock()
	ts.members[teamID] = append(ts.members[teamID], store.TeamMemberData{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember})
	ts.mu.Unlock()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("DELETE", "/v1/teams/"+teamID.String()+"/members/"+memberID.String(), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	ts.mu.Lock()
	if len(ts.members[teamID]) != 1 {
		t.Errorf("members = %d, want 1", len(ts.members[teamID]))
	}
	ts.mu.Unlock()
}

func TestTeamsRemoveMember_ByAgentKey(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)
	ts.mu.Lock()
	ts.members[teamID] = append(ts.members[teamID], store.TeamMemberData{TeamID: teamID, AgentID: memberID, Role: store.TeamRoleMember})
	ts.mu.Unlock()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("DELETE", "/v1/teams/"+teamID.String()+"/members/member-agent", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestTeamsRemoveMember_CannotRemoveLead(t *testing.T) {
	setupTestCache(t, nil)
	h, ts, _, _ := newTestTeamsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	teamID := uuid.New()
	seedTeam(ts, teamID)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, teamReq("DELETE", "/v1/teams/"+teamID.String()+"/members/"+leadID.String(), nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
