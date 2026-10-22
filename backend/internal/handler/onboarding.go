package handler

import (
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/access"
)

func (h *Handler) onboarding(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "onboarding access", err)
		return
	}
	p, err := h.q.OnboardingProgress(r.Context(), me.ID)
	if err != nil {
		internalError(w, "onboarding progress", err)
		return
	}
	computers, err := h.q.ListPersonComputers(r.Context(), me.ID)
	if err != nil {
		internalError(w, "onboarding computers", err)
		return
	}
	connected := false
	for _, c := range computers {
		connected = connected || c.Connected
	}
	fi, err := os.Stat(h.cfg.AgentBinaryPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"company_name": p.CompanyName, "can_manage": acc.Has(access.ManageRoles),
		"agent_available":    err == nil && fi.Mode().IsRegular(),
		"computer_connected": connected, "has_apps": p.HasApps,
		"has_successful_run": p.HasSuccessfulRun, "has_team": p.HasTeam,
	})
}

func (h *Handler) updateOnboarding(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var b struct {
		CompanyName string `json:"company_name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	b.CompanyName = strings.TrimSpace(b.CompanyName)
	if b.CompanyName == "" || utf8.RuneCountInString(b.CompanyName) > 120 {
		writeError(w, http.StatusBadRequest, "Informe o nome da empresa com até 120 caracteres.")
		return
	}
	if err := h.q.SetCompanyName(r.Context(), b.CompanyName); err != nil {
		internalError(w, "company name", err)
		return
	}
	h.onboarding(w, r)
}
