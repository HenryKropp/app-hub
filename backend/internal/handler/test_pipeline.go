package handler

import (
	"context"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// advanceDraft walks draft d one stage forward (filled in Task 5).
func (h *Handler) advanceDraft(ctx context.Context, d sqlc.AppDraft, now time.Time) error {
	return nil
}
