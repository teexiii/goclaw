package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
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

	// Resolve lead agent.
	leadAgent, err := h.resolveAgent(ctx, req.Lead)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", req.Lead)})
		return
	}

	// Enforce single-team leadership.
	if existing, _ := h.teamStore.GetTeamForAgent(ctx, leadAgent.ID); existing != nil && existing.LeadAgentID == leadAgent.ID {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("agent %q already leads team %q", req.Lead, existing.Name),
		})
		return
	}

	// Resolve member agents, deduplicating and excluding lead.
	seen := map[string]bool{leadAgent.ID.String(): true}
	var memberAgents []*store.AgentData
	for _, key := range req.Members {
		ag, err := h.resolveAgent(ctx, key)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", key)})
			return
		}
		if seen[ag.ID.String()] {
			continue // skip lead and duplicates
		}
		seen[ag.ID.String()] = true
		memberAgents = append(memberAgents, ag)
	}

	// Create team.
	team := &store.TeamData{
		Name:        req.Name,
		LeadAgentID: leadAgent.ID,
		Description: req.Description,
		Status:      store.TeamStatusActive,
		Settings:    req.Settings,
		CreatedBy:   userID,
	}
	if err := h.teamStore.CreateTeam(ctx, team); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "team", err.Error())})
		return
	}

	// Add lead as member.
	if err := h.teamStore.AddMember(ctx, team.ID, leadAgent.ID, store.TeamRoleLead); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "team lead", err.Error())})
		return
	}

	// Add members (already deduplicated, lead excluded).
	for _, ag := range memberAgents {
		if err := h.teamStore.AddMember(ctx, team.ID, ag.ID, store.TeamRoleMember); err != nil {
			slog.Warn("teams.create: failed to add member", "agent", ag.AgentKey, "team", team.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToCreate, "member "+ag.AgentKey, err.Error())})
			return
		}
	}

	// Auto-create outbound links from lead to each member.
	if h.linkStore != nil {
		for _, member := range memberAgents {
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
			if err := h.linkStore.CreateLink(ctx, link); err != nil {
				slog.Debug("teams.create: auto-link exists or failed", "source", leadAgent.AgentKey, "target", member.AgentKey, "error", err)
			}
		}
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "team.created", "team", team.ID.String())

	if h.msgBus != nil {
		h.msgBus.Broadcast(bus.Event{
			Name: protocol.EventTeamCreated,
			Payload: protocol.TeamCreatedPayload{
				TeamID:          team.ID.String(),
				TeamName:        team.Name,
				LeadAgentKey:    leadAgent.AgentKey,
				LeadDisplayName: leadAgent.DisplayName,
				MemberCount:     1 + len(memberAgents),
			},
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"team": team})
}
