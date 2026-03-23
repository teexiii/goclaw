package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}
	if req.Agent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "agent")})
		return
	}

	// Validate team exists.
	team, err := h.teamStore.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}

	// Resolve agent.
	ag, err := h.resolveAgent(ctx, req.Agent)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", req.Agent)})
		return
	}

	// Prevent adding lead again.
	if ag.ID == team.LeadAgentID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgAgentIsTeamLead)})
		return
	}

	// Check if already a member.
	members, err := h.teamStore.ListMembers(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "members")})
		return
	}
	for _, m := range members {
		if m.AgentID == ag.ID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": i18n.T(locale, i18n.MsgAlreadyExists, "member", ag.AgentKey)})
			return
		}
	}

	// Validate role.
	role := req.Role
	switch role {
	case store.TeamRoleMember, store.TeamRoleReviewer:
		// valid
	case "":
		role = store.TeamRoleMember
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, "role must be member or reviewer")})
		return
	}

	if err := h.teamStore.AddMember(ctx, teamID, ag.ID, role); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "member", err.Error())})
		return
	}

	// Auto-create link from lead to new member.
	if h.linkStore != nil {
		leadAgent, err := h.agentStore.GetByID(ctx, team.LeadAgentID)
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
			if err := h.linkStore.CreateLink(ctx, link); err != nil {
				slog.Debug("teams.members.add: auto-link exists or failed", "source", leadAgent.AgentKey, "target", ag.AgentKey, "error", err)
			}
		}
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.member_added", "team", teamID.String())

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

	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (h *TeamsHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)

	teamID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "team")})
		return
	}

	// Resolve agent: accept UUID or agent_key in path.
	var agentID uuid.UUID
	if id, err := uuid.Parse(r.PathValue("agentId")); err == nil {
		agentID = id
	} else {
		ag, agErr := h.agentStore.GetByKey(ctx, r.PathValue("agentId"))
		if agErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", r.PathValue("agentId"))})
			return
		}
		agentID = ag.ID
	}

	// Validate team exists and prevent removing the lead.
	team, err := h.teamStore.GetTeam(ctx, teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "team", teamID.String())})
		return
	}
	if agentID == team.LeadAgentID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgCannotRemoveTeamLead)})
		return
	}

	// Fetch agent info before removal for event emission.
	removedAgent, _ := h.agentStore.GetByID(ctx, agentID)

	if err := h.teamStore.RemoveMember(ctx, teamID, agentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToDelete, "member", err.Error())})
		return
	}

	// Clean up team-specific links.
	if h.linkStore != nil {
		if err := h.linkStore.DeleteTeamLinksForAgent(ctx, teamID, agentID); err != nil {
			slog.Warn("teams.members.remove: failed to clean up links", "team", teamID, "agent", agentID, "error", err)
		}
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.member_removed", "team", teamID.String())

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
