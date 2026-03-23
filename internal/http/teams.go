package http

import (
	"context"
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
	teamStore  store.TeamStore
	agentStore store.AgentStore
	linkStore  store.AgentLinkStore
	token      string
	msgBus     *bus.MessageBus
}

// NewTeamsHandler creates a handler for team management endpoints.
func NewTeamsHandler(teamStore store.TeamStore, agentStore store.AgentStore, linkStore store.AgentLinkStore, token string, msgBus *bus.MessageBus) *TeamsHandler {
	return &TeamsHandler{teamStore: teamStore, agentStore: agentStore, linkStore: linkStore, token: token, msgBus: msgBus}
}

// RegisterRoutes registers team management routes on the given mux.
func (h *TeamsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/teams", h.auth(h.handleCreate))
	mux.HandleFunc("GET /v1/teams", h.auth(h.handleList))
	mux.HandleFunc("GET /v1/teams/{id}", h.auth(h.handleGet))
	mux.HandleFunc("POST /v1/teams/{id}/members", h.auth(h.handleAddMember))
	mux.HandleFunc("DELETE /v1/teams/{id}/members/{agentId}", h.auth(h.handleRemoveMember))
}

func (h *TeamsHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

func (h *TeamsHandler) emitCacheInvalidate() {
	if h.msgBus == nil {
		return
	}
	h.msgBus.Broadcast(bus.Event{
		Name:    protocol.EventCacheInvalidate,
		Payload: bus.CacheInvalidatePayload{Kind: bus.CacheKindTeam},
	})
}

// resolveAgent resolves an agent by UUID string or agent_key.
func (h *TeamsHandler) resolveAgent(ctx context.Context, keyOrID string) (*store.AgentData, error) {
	if id, err := uuid.Parse(keyOrID); err == nil {
		return h.agentStore.GetByID(ctx, id)
	}
	return h.agentStore.GetByKey(ctx, keyOrID)
}

// --- List ---

func (h *TeamsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	userID := store.UserIDFromContext(ctx)
	auth := resolveAuth(r, h.token)

	var teams []store.TeamData
	var err error
	if permissions.HasMinRole(auth.Role, permissions.RoleAdmin) {
		teams, err = h.teamStore.ListTeams(ctx)
	} else {
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgUserIDHeader)})
			return
		}
		teams, err = h.teamStore.ListUserTeams(ctx, userID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "teams")})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"teams": teams, "count": len(teams)})
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

	team, err := h.teamStore.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}

	// Non-admin callers must have team access.
	auth := resolveAuth(r, h.token)
	if !permissions.HasMinRole(auth.Role, permissions.RoleAdmin) {
		callerID := store.UserIDFromContext(ctx)
		if ok, err := h.teamStore.HasTeamAccess(ctx, teamID, callerID); err != nil || !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
			return
		}
	}

	members, err := h.teamStore.ListMembers(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "members")})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team": team, "members": members})
}
