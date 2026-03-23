package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// TeamsHandler handles team CRUD and member management HTTP endpoints.
type TeamsHandler struct {
	teams  store.TeamStore
	agents store.AgentStore
	links  store.AgentLinkStore
	token  string
	msgBus *bus.MessageBus
}

// NewTeamsHandler creates a handler for team management endpoints.
func NewTeamsHandler(teams store.TeamStore, agents store.AgentStore, links store.AgentLinkStore, token string, msgBus *bus.MessageBus) *TeamsHandler {
	return &TeamsHandler{teams: teams, agents: agents, links: links, token: token, msgBus: msgBus}
}

// RegisterRoutes registers all team management routes on the given mux.
func (h *TeamsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams", h.authMiddleware(h.handleList))
	mux.HandleFunc("POST /v1/teams", h.authMiddleware(h.handleCreate))
	mux.HandleFunc("GET /v1/teams/{id}", h.authMiddleware(h.handleGet))
	mux.HandleFunc("POST /v1/teams/{id}/members", h.authMiddleware(h.handleAddMember))
	mux.HandleFunc("DELETE /v1/teams/{id}/members/{agentId}", h.authMiddleware(h.handleRemoveMember))
}

func (h *TeamsHandler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

// emitCacheInvalidate broadcasts a cache invalidation event for team data.
func (h *TeamsHandler) emitCacheInvalidate() {
	if h.msgBus == nil {
		return
	}
	h.msgBus.Broadcast(bus.Event{
		Name:    protocol.EventCacheInvalidate,
		Payload: bus.CacheInvalidatePayload{Kind: bus.CacheKindTeam},
	})
}

// resolveAgent resolves an agent by UUID or agent_key.
func (h *TeamsHandler) resolveAgent(ctx context.Context, keyOrID string) (*store.AgentData, error) {
	if id, err := uuid.Parse(keyOrID); err == nil {
		return h.agents.GetByID(ctx, id)
	}
	return h.agents.GetByKey(ctx, keyOrID)
}

// --- List ---

func (h *TeamsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	userID := store.UserIDFromContext(r.Context())
	auth := resolveAuth(r, h.token)

	var teams []store.TeamData
	var err error
	if permissions.HasMinRole(auth.Role, permissions.RoleAdmin) {
		teams, err = h.teams.ListTeams(r.Context())
	} else {
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgUserIDHeader)})
			return
		}
		teams, err = h.teams.ListUserTeams(r.Context(), userID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"teams": teams, "count": len(teams)})
}

// --- Create ---

type teamCreateRequest struct {
	Name        string          `json:"name"`
	Lead        string          `json:"lead"`    // agent key or UUID
	Members     []string        `json:"members"` // agent keys or UUIDs
	Description string          `json:"description"`
	Settings    json.RawMessage `json:"settings"`
}

func (h *TeamsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	userID := store.UserIDFromContext(ctx)
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgUserIDHeader)})
		return
	}

	var req teamCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, err.Error())})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "name")})
		return
	}
	if req.Lead == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "lead")})
		return
	}

	// Resolve lead agent
	leadAgent, err := h.resolveAgent(ctx, req.Lead)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lead agent: " + err.Error()})
		return
	}

	// Enforce single-team leadership
	if existingTeam, _ := h.teams.GetTeamForAgent(ctx, leadAgent.ID); existingTeam != nil && existingTeam.LeadAgentID == leadAgent.ID {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("agent %q already leads team %q", req.Lead, existingTeam.Name),
		})
		return
	}

	// Resolve member agents
	var memberAgents []*store.AgentData
	for _, memberKey := range req.Members {
		ag, err := h.resolveAgent(ctx, memberKey)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "member agent " + memberKey + ": " + err.Error()})
			return
		}
		memberAgents = append(memberAgents, ag)
	}

	// Create team
	team := &store.TeamData{
		Name:        req.Name,
		LeadAgentID: leadAgent.ID,
		Description: req.Description,
		Status:      store.TeamStatusActive,
		Settings:    req.Settings,
		CreatedBy:   userID,
	}
	if err := h.teams.CreateTeam(ctx, team); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "team", err.Error())})
		return
	}

	// Add lead as member with lead role
	if err := h.teams.AddMember(ctx, team.ID, leadAgent.ID, store.TeamRoleLead); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "team lead membership", err.Error())})
		return
	}

	// Add members
	for _, ag := range memberAgents {
		if ag.ID == leadAgent.ID {
			continue
		}
		_ = h.teams.AddMember(ctx, team.ID, ag.ID, store.TeamRoleMember)
	}

	// Auto-create outbound agent_links from lead to each member
	if h.links != nil {
		for _, member := range memberAgents {
			if member.ID == leadAgent.ID {
				continue
			}
			link := &store.AgentLinkData{
				SourceAgentID: leadAgent.ID,
				TargetAgentID: member.ID,
				Direction:     store.LinkDirectionOutbound,
				TeamID:        &team.ID,
				Description:   "auto-created by team",
				MaxConcurrent: 3,
				Status:        store.LinkStatusActive,
				CreatedBy:     userID,
			}
			_ = h.links.CreateLink(ctx, link)
		}
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.created", "team", team.ID.String())

	// Emit team.created event
	if h.msgBus != nil {
		h.msgBus.Broadcast(bus.Event{
			Name: protocol.EventTeamCreated,
			Payload: protocol.TeamCreatedPayload{
				TeamID:          team.ID.String(),
				TeamName:        team.Name,
				LeadAgentKey:    leadAgent.AgentKey,
				LeadDisplayName: leadAgent.DisplayName,
				MemberCount:     len(memberAgents) + 1,
			},
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"team": team})
}

// --- Get ---

func (h *TeamsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)

	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "team")})
		return
	}

	team, err := h.teams.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}

	// Non-admin callers must have team access
	auth := resolveAuth(r, h.token)
	if !permissions.HasMinRole(auth.Role, permissions.RoleAdmin) {
		callerID := store.UserIDFromContext(ctx)
		if ok, err := h.teams.HasTeamAccess(ctx, teamID, callerID); err != nil || !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
			return
		}
	}

	members, err := h.teams.ListMembers(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team": team, "members": members})
}

// --- Add Member ---

type addMemberRequest struct {
	Agent string `json:"agent"` // agent key or UUID
	Role  string `json:"role"`  // "member" (default) or "reviewer"
}

func (h *TeamsHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	userID := store.UserIDFromContext(ctx)

	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "team")})
		return
	}

	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, err.Error())})
		return
	}
	if req.Agent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "agent")})
		return
	}

	// Validate team exists
	team, err := h.teams.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}

	// Resolve agent
	ag, err := h.resolveAgent(ctx, req.Agent)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent: " + err.Error()})
		return
	}

	// Prevent adding lead again
	if ag.ID == team.LeadAgentID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgAgentIsTeamLead)})
		return
	}

	// Validate and default role
	role := req.Role
	switch role {
	case store.TeamRoleMember, store.TeamRoleReviewer:
		// valid
	case "":
		role = store.TeamRoleMember
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be member or reviewer"})
		return
	}

	if err := h.teams.AddMember(ctx, teamID, ag.ID, role); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "member", err.Error())})
		return
	}

	// Auto-create link from lead to new member
	if h.links != nil {
		leadAgent, err := h.agents.GetByID(ctx, team.LeadAgentID)
		if err == nil {
			link := &store.AgentLinkData{
				SourceAgentID: leadAgent.ID,
				TargetAgentID: ag.ID,
				Direction:     store.LinkDirectionOutbound,
				TeamID:        &teamID,
				Description:   "auto-created by team",
				MaxConcurrent: 3,
				Status:        store.LinkStatusActive,
				CreatedBy:     userID,
			}
			_ = h.links.CreateLink(ctx, link)
		}
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.member_added", "team", teamID.String())

	// Emit event
	if h.msgBus != nil {
		h.msgBus.Broadcast(bus.Event{
			Name: protocol.EventTeamMemberAdded,
			Payload: protocol.TeamMemberAddedPayload{
				TeamID:      teamID.String(),
				TeamName:    team.Name,
				AgentID:     ag.ID.String(),
				AgentKey:    ag.AgentKey,
				DisplayName: ag.DisplayName,
				Role:        role,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// --- Remove Member ---

func (h *TeamsHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)

	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "team")})
		return
	}

	// Resolve agent: accept UUID or agent_key in path
	var agentID uuid.UUID
	if id, err := uuid.Parse(r.PathValue("agentId")); err == nil {
		agentID = id
	} else {
		ag, agErr := h.agents.GetByKey(ctx, r.PathValue("agentId"))
		if agErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "agent")})
			return
		}
		agentID = ag.ID
	}

	// Validate team exists and prevent removing the lead
	team, err := h.teams.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}
	if agentID == team.LeadAgentID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgCannotRemoveTeamLead)})
		return
	}

	// Fetch agent info before removal for event
	removedAgent, _ := h.agents.GetByID(ctx, agentID)

	if err := h.teams.RemoveMember(ctx, teamID, agentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToDelete, "member", err.Error())})
		return
	}

	// Clean up team-specific links
	if h.links != nil {
		_ = h.links.DeleteTeamLinksForAgent(ctx, teamID, agentID)
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.member_removed", "team", teamID.String())

	// Emit event
	if h.msgBus != nil && removedAgent != nil {
		h.msgBus.Broadcast(bus.Event{
			Name: protocol.EventTeamMemberRemoved,
			Payload: protocol.TeamMemberRemovedPayload{
				TeamID:      teamID.String(),
				TeamName:    team.Name,
				AgentID:     removedAgent.ID.String(),
				AgentKey:    removedAgent.AgentKey,
				DisplayName: removedAgent.DisplayName,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
