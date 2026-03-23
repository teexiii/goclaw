package http

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// allowedAgentFiles is the list of context files exposed via HTTP.
var allowedAgentFiles = []string{
	bootstrap.AgentsFile, bootstrap.SoulFile, bootstrap.IdentityFile,
	bootstrap.UserFile, bootstrap.UserPredefinedFile, bootstrap.BootstrapFile,
	bootstrap.MemoryJSONFile, bootstrap.HeartbeatFile,
}

// resolveAgentForFiles resolves an agent by UUID or key, with access check.
func (h *AgentsHandler) resolveAgentForFiles(w http.ResponseWriter, r *http.Request) *store.AgentData {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	userID := store.UserIDFromContext(ctx)
	idStr := r.PathValue("id")

	var ag *store.AgentData
	var err error
	if id, parseErr := uuid.Parse(idStr); parseErr == nil {
		ag, err = h.agents.GetByID(ctx, id)
	} else {
		ag, err = h.agents.GetByKey(ctx, idStr)
	}
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "agent", idStr)})
		return nil
	}

	// Access check for non-owners
	if userID != "" && !h.isOwnerUser(userID) {
		if ok, _, _ := h.agents.CanAccess(ctx, ag.ID, userID); !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": i18n.T(locale, i18n.MsgNoAccess, "agent")})
			return nil
		}
	}
	return ag
}

// handleListFiles returns the list of agent context files.
func (h *AgentsHandler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	ag := h.resolveAgentForFiles(w, r)
	if ag == nil {
		return
	}
	locale := store.LocaleFromContext(r.Context())

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
			files = append(files, map[string]any{"name": name, "size": len(f.Content), "missing": false})
		} else {
			files = append(files, map[string]any{"name": name, "missing": true})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"agentId": ag.AgentKey, "files": files})
}

// handleGetFile returns the content of a single agent context file.
func (h *AgentsHandler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	ag := h.resolveAgentForFiles(w, r)
	if ag == nil {
		return
	}
	locale := store.LocaleFromContext(r.Context())
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
				"agentId": ag.AgentKey,
				"file":    map[string]any{"name": fileName, "content": f.Content, "size": len(f.Content), "missing": false},
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agentId": ag.AgentKey,
		"file":    map[string]any{"name": fileName, "missing": true},
	})
}

// handleSetFile writes content to an agent context file.
func (h *AgentsHandler) handleSetFile(w http.ResponseWriter, r *http.Request) {
	ag := h.resolveAgentForFiles(w, r)
	if ag == nil {
		return
	}
	locale := store.LocaleFromContext(r.Context())
	fileName := r.PathValue("fileName")

	if !slices.Contains(allowedAgentFiles, fileName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidRequest, "file not allowed: "+fileName)})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	if err := h.agents.SetAgentContextFile(r.Context(), ag.ID, fileName, body.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToSave, "file", err.Error())})
		return
	}

	h.emitCacheInvalidate(bus.CacheKindBootstrap, ag.ID.String())
	emitAudit(h.msgBus, r, "agent.file_set", "agent", ag.ID.String())

	writeJSON(w, http.StatusOK, map[string]any{
		"agentId": ag.AgentKey,
		"file":    map[string]any{"name": fileName, "content": body.Content, "size": len(body.Content), "missing": false},
	})
}
