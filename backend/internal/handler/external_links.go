package handler

// External links (ADR 045): the team creates, lists and cancels links that
// open an HTML artifact for someone outside the company, without a login.
// The pages those people see are in external_page.go.

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/extlink"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const msgNoShareExternal = "Você não tem permissão para compartilhar fora da empresa."

type linkJSON struct {
	ID            int64      `json:"id"`
	URL           string     `json:"url"`
	Kind          string     `json:"kind"`
	Recipient     string     `json:"recipient"`
	Content       string     `json:"content"`
	VersionNumber int32      `json:"version_number"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	Status        string     `json:"status"`
	Opens         int32      `json:"opens"`
	LastOpenedAt  *time.Time `json:"last_opened_at"`
	ArtifactID    int64      `json:"artifact_id"`
	ArtifactName  string     `json:"artifact_name,omitempty"`
}

type artifactLinksJSON struct {
	EmailAvailable bool       `json:"email_available"`
	CanShare       bool       `json:"can_share"`
	Links          []linkJSON `json:"links"`
}

type companyLinksJSON struct {
	Links []linkJSON `json:"links"`
}

func (h *Handler) registerExternalLinks(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artifacts/{id}/links", h.listArtifactLinks)
	mux.HandleFunc("POST /api/artifacts/{id}/links", h.createArtifactLink)
	mux.HandleFunc("POST /api/links/{id}/cancel", h.cancelLink)
	mux.HandleFunc("GET /api/links", h.listCompanyLinks)
}

func (h *Handler) companySlug() string {
	if h.workspace != nil && h.workspace.Slug != "" {
		return h.workspace.Slug
	}
	return "principal"
}

// linkURL is the address people outside the company open.
func (h *Handler) linkURL(token string) string {
	scheme := "http"
	if h.cfg.SecureCookies() {
		scheme = "https"
	}
	return scheme + "://link." + h.cfg.SharedDomain + "/" + h.companySlug() + "/" + token
}

// linkState gathers what decides whether l opens now: the creator's current
// access and the kind of the version it shows.
func (h *Handler) linkState(ctx context.Context, l sqlc.ExternalLink, now time.Time) (extlink.State, sqlc.ArtifactVersion, error) {
	s := extlink.State{Now: now, ExpiresAt: l.ExpiresAt.Time, Cancelled: l.CancelledAt.Valid}
	a, err := h.q.GetArtifact(ctx, l.ArtifactID)
	if err != nil {
		return s, sqlc.ArtifactVersion{}, err
	}
	if !l.CreatedBy.Valid {
		s.CreatorGone = true
	} else if p, err := h.q.GetPerson(ctx, l.CreatedBy.Int64); errors.Is(err, pgx.ErrNoRows) {
		s.CreatorGone = true
	} else if err != nil {
		return s, sqlc.ArtifactVersion{}, err
	} else {
		acc, err := h.loadAccess(ctx, p)
		if err != nil {
			return s, sqlc.ArtifactVersion{}, err
		}
		s.CreatorCanShare = acc.Has(access.ShareExternal)
		s.CreatorSeesArtifact = acc.ForArtifact(artifactRef(a)).View
	}
	var v sqlc.ArtifactVersion
	if l.VersionID.Valid {
		v, err = h.q.GetArtifactVersionByID(ctx, l.VersionID.Int64)
	} else {
		// "A mais recente" outside is the newest approved one (ADR 046).
		v, err = h.q.LatestApprovedArtifactVersion(ctx, l.ArtifactID)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return s, v, err
	}
	s.ContentIsFile = err == nil && v.ArtifactID == l.ArtifactID && v.Approved && v.Kind == "file" && v.BlobSha256.Valid
	return s, v, nil
}

func linkStatus(s extlink.State) string {
	switch {
	case s.Cancelled:
		return "cancelado"
	case !s.Now.Before(s.ExpiresAt):
		return "vencido"
	case !extlink.Open(s):
		return "fechado"
	}
	return "ativo"
}

func (h *Handler) toLinkJSON(ctx context.Context, l sqlc.ExternalLink, createdBy string, version pgtype.Int4, opens int32, last pgtype.Timestamptz, now time.Time) (linkJSON, error) {
	s, _, err := h.linkState(ctx, l, now)
	if err != nil {
		return linkJSON{}, err
	}
	out := linkJSON{ID: l.ID, URL: h.linkURL(l.Token), Kind: l.Kind, Recipient: l.RecipientEmail.String, Content: "latest",
		CreatedByName: createdBy, CreatedAt: l.CreatedAt.Time, ExpiresAt: l.ExpiresAt.Time, Status: linkStatus(s),
		Opens: opens, ArtifactID: l.ArtifactID}
	if l.VersionID.Valid {
		out.Content, out.VersionNumber = "version", version.Int32
	}
	if last.Valid {
		t := last.Time
		out.LastOpenedAt = &t
	}
	return out, nil
}

func (h *Handler) listArtifactLinks(w http.ResponseWriter, r *http.Request) {
	a, _, acc, _, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	out := artifactLinksJSON{EmailAvailable: h.mail != nil, CanShare: acc.Has(access.ShareExternal) && a.Kind == "file", Links: []linkJSON{}}
	if !acc.Has(access.ShareExternal) {
		writeJSON(w, http.StatusOK, out)
		return
	}
	rows, err := h.q.ListArtifactExternalLinks(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list artifact links", err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		l := sqlc.ExternalLink{ID: row.ID, Token: row.Token, ArtifactID: row.ArtifactID, VersionID: row.VersionID, Kind: row.Kind,
			RecipientEmail: row.RecipientEmail, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			CancelledAt: row.CancelledAt, CancelledBy: row.CancelledBy}
		j, err := h.toLinkJSON(r.Context(), l, row.CreatedByName, row.VersionNumber, row.Opens, row.LastOpenedAt, now)
		if err != nil {
			internalError(w, "link state", err)
			return
		}
		out.Links = append(out.Links, j)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listCompanyLinks(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !acc.Has(access.ManageRoles) {
		writeError(w, http.StatusForbidden, "Só quem cuida dos cargos vê os links da empresa.")
		return
	}
	rows, err := h.q.ListCompanyExternalLinks(r.Context())
	if err != nil {
		internalError(w, "list company links", err)
		return
	}
	out := companyLinksJSON{Links: []linkJSON{}}
	now := time.Now()
	for _, row := range rows {
		l := sqlc.ExternalLink{ID: row.ID, Token: row.Token, ArtifactID: row.ArtifactID, VersionID: row.VersionID, Kind: row.Kind,
			RecipientEmail: row.RecipientEmail, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			CancelledAt: row.CancelledAt, CancelledBy: row.CancelledBy}
		j, err := h.toLinkJSON(r.Context(), l, row.CreatedByName, row.VersionNumber, row.Opens, row.LastOpenedAt, now)
		if err != nil {
			internalError(w, "link state", err)
			return
		}
		j.ArtifactName = row.ArtifactName
		out.Links = append(out.Links, j)
	}
	writeJSON(w, http.StatusOK, out)
}

type createLinkInput struct {
	Kind        string `json:"kind"`
	Email       string `json:"email"`
	Content     string `json:"content"`
	Version     int32  `json:"version"`
	ExpiresDays int    `json:"expires_days"`
}

func (h *Handler) createArtifactLink(w http.ResponseWriter, r *http.Request) {
	a, me, acc, _, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !acc.Has(access.ShareExternal) {
		writeError(w, http.StatusForbidden, msgNoShareExternal)
		return
	}
	var in createLinkInput
	if !readJSON(w, r, &in) {
		return
	}
	if a.Kind != "file" {
		writeError(w, http.StatusBadRequest, "Só artifacts de arquivo HTML podem ser mandados para fora. Para um link, mande o próprio endereço.")
		return
	}
	if !a.EverApproved {
		writeError(w, http.StatusConflict, "Este artifact ainda espera aprovação.")
		return
	}
	if !extlink.ValidExpiry(in.ExpiresDays) {
		writeError(w, http.StatusBadRequest, "Escolha um prazo: 7 dias, 30 dias, 90 dias ou 1 ano.")
		return
	}
	params := sqlc.CreateExternalLinkParams{Token: extlink.NewToken(), ArtifactID: a.ID, Kind: in.Kind,
		CreatedBy: pgtype.Int8{Int64: me.ID, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(in.ExpiresDays) * 24 * time.Hour), Valid: true}}
	switch in.Kind {
	case "open":
	case "email":
		if h.mail == nil {
			writeError(w, http.StatusBadRequest, "O envio de e-mail ainda não está ligado neste Ziv.")
			return
		}
		email := strings.ToLower(strings.TrimSpace(in.Email))
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			writeError(w, http.StatusBadRequest, "Escreva um e-mail válido para quem vai receber.")
			return
		}
		params.RecipientEmail = pgtype.Text{String: email, Valid: true}
	default:
		writeError(w, http.StatusBadRequest, "Escolha o tipo do link: código por e-mail ou link aberto.")
		return
	}
	var versionNumber pgtype.Int4
	switch in.Content {
	case "latest":
	case "version":
		v, err := h.q.GetArtifactVersion(r.Context(), sqlc.GetArtifactVersionParams{ArtifactID: a.ID, Number: in.Version})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && v.Kind != "file") {
			writeError(w, http.StatusBadRequest, "Essa versão não existe ou não é um arquivo.")
			return
		}
		if err != nil {
			internalError(w, "get version", err)
			return
		}
		params.VersionID = pgtype.Int8{Int64: v.ID, Valid: true}
		versionNumber = pgtype.Int4{Int32: v.Number, Valid: true}
	default:
		writeError(w, http.StatusBadRequest, "Escolha o conteúdo: sempre a versão mais recente ou uma versão fixa.")
		return
	}
	l, err := h.q.CreateExternalLink(r.Context(), params)
	if err != nil {
		internalError(w, "create link", err)
		return
	}
	j, err := h.toLinkJSON(r.Context(), l, me.Name, versionNumber, 0, pgtype.Timestamptz{}, time.Now())
	if err != nil {
		internalError(w, "link state", err)
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

func (h *Handler) cancelLink(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Link não encontrado.")
		return
	}
	l, err := h.q.GetExternalLink(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Link não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get link", err)
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	a, err := h.q.GetArtifact(r.Context(), l.ArtifactID)
	if err != nil {
		internalError(w, "get artifact", err)
		return
	}
	// Admins (Admin list) cancel any link; otherwise share_external and
	// seeing the artifact.
	if !acc.Has(access.ManageRoles) && !(acc.Has(access.ShareExternal) && acc.ForArtifact(artifactRef(a)).View) {
		writeError(w, http.StatusForbidden, "Você não pode cancelar este link.")
		return
	}
	if _, err := h.q.CancelExternalLink(r.Context(), sqlc.CancelExternalLinkParams{ID: l.ID, CancelledBy: pgtype.Int8{Int64: me.ID, Valid: true}}); err != nil {
		internalError(w, "cancel link", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
