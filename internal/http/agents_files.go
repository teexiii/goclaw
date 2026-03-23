package http

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// allowedAgentFiles is the list of context files exposed via the HTTP API.
var allowedAgentFiles = []string{
	bootstrap.AgentsFile, bootstrap.SoulFile, bootstrap.IdentityFile,
	bootstrap.UserFile, bootstrap.UserPredefinedFile, bootstrap.BootstrapFile,
	bootstrap.MemoryJSONFile, bootstrap.HeartbeatFile,
}

// handleListFiles lists all agent-level context files.
// GET /v1/agents/{id}/files
func (h *AgentsHandler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	ag, ok := h.resolveAgentWithAccess(w, r)
	if !ok {
		return
	}

	dbFiles, err := h.agents.GetAgentContextFiles(r.Context(), ag.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "files")})
		return
	}

	dbMap := make(map[string]store.AgentContextFileData, len(dbFiles))
	for _, f := range dbFiles {
		dbMap[f.FileName] = f
	}

	files := make([]map[string]any, 0, len(allowedAgentFiles))
	for _, name := range allowedAgentFiles {
		if f, ok := dbMap[name]; ok {
			files = append(files, map[string]any{
				"name":    name,
				"missing": false,
				"size":    len(f.Content),
			})
		} else {
			files = append(files, map[string]any{
				"name":    name,
				"missing": true,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  ag.ID,
		"agent_key": ag.AgentKey,
		"files":     files,
	})
}

// handleGetFile reads a single agent-level context file.
// GET /v1/agents/{id}/files/{fileName}
func (h *AgentsHandler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	ag, ok := h.resolveAgentWithAccess(w, r)
	if !ok {
		return
	}

	fileName := r.PathValue("fileName")
	if !slices.Contains(allowedAgentFiles, fileName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, "file not allowed: "+fileName)})
		return
	}

	dbFiles, err := h.agents.GetAgentContextFiles(r.Context(), ag.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "files")})
		return
	}

	for _, f := range dbFiles {
		if f.FileName == fileName {
			writeJSON(w, http.StatusOK, map[string]any{
				"agent_id":  ag.ID,
				"agent_key": ag.AgentKey,
				"file": map[string]any{
					"name":    fileName,
					"missing": false,
					"size":    len(f.Content),
					"content": f.Content,
				},
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  ag.ID,
		"agent_key": ag.AgentKey,
		"file": map[string]any{
			"name":    fileName,
			"missing": true,
		},
	})
}

// handleSetFile writes a single agent-level context file.
// PUT /v1/agents/{id}/files/{fileName}
func (h *AgentsHandler) handleSetFile(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	ag, ok := h.resolveAgentOwnerOnly(w, r)
	if !ok {
		return
	}

	fileName := r.PathValue("fileName")
	if !slices.Contains(allowedAgentFiles, fileName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, "file not allowed: "+fileName)})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, err.Error())})
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	if err := h.agents.SetAgentContextFile(r.Context(), ag.ID, fileName, payload.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToSave, "file", err.Error())})
		return
	}

	// Invalidate caches so the agent picks up the change immediately
	h.emitCacheInvalidate(bus.CacheKindAgent, ag.AgentKey)
	h.emitCacheInvalidate(bus.CacheKindBootstrap, ag.ID.String())

	emitAudit(h.msgBus, r, "agent.file_set", "agent", ag.ID.String())
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  ag.ID,
		"agent_key": ag.AgentKey,
		"file": map[string]any{
			"name":    fileName,
			"size":    len(payload.Content),
			"content": payload.Content,
		},
	})
}

// resolveAgentWithAccess parses the agent ID from the path, checks access, and returns the agent.
// Returns false if an error response was already written.
func (h *AgentsHandler) resolveAgentWithAccess(w http.ResponseWriter, r *http.Request) (*store.AgentData, bool) {
	userID := store.UserIDFromContext(r.Context())
	locale := store.LocaleFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "agent")})
		return nil, false
	}

	ag, err := h.agents.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", id.String())})
		return nil, false
	}

	if userID != "" && !h.isOwnerUser(userID) {
		if ok, _, _ := h.agents.CanAccess(r.Context(), id, userID); !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": i18n.T(locale, i18n.MsgNoAccess, "agent")})
			return nil, false
		}
	}

	return ag, true
}

// resolveAgentOwnerOnly parses the agent ID and checks that the caller is the owner.
func (h *AgentsHandler) resolveAgentOwnerOnly(w http.ResponseWriter, r *http.Request) (*store.AgentData, bool) {
	userID := store.UserIDFromContext(r.Context())
	locale := store.LocaleFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "agent")})
		return nil, false
	}

	ag, err := h.agents.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", id.String())})
		return nil, false
	}

	if userID != "" && ag.OwnerID != userID && !h.isOwnerUser(userID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": i18n.T(locale, i18n.MsgOwnerOnly, "edit agent files")})
		return nil, false
	}

	return ag, true
}
