package sqlc

import "context"

// OnboardingProgress reads current facts, rather than storing checked boxes.
type OnboardingProgress struct {
	CompanyName                        string
	HasApps, HasSuccessfulRun, HasTeam bool
}

func (q *Queries) OnboardingProgress(ctx context.Context, personID int64) (p OnboardingProgress, err error) {
	err = q.db.QueryRow(ctx, `SELECT company_name,
		EXISTS(SELECT 1 FROM apps),
		EXISTS(SELECT 1 FROM runs WHERE person_id=$1 AND status='done'),
		(SELECT count(*) > 1 FROM people)
		FROM hub_settings WHERE id=TRUE`, personID).Scan(&p.CompanyName, &p.HasApps, &p.HasSuccessfulRun, &p.HasTeam)
	return
}

func (q *Queries) SetCompanyName(ctx context.Context, name string) error {
	_, err := q.db.Exec(ctx, `UPDATE hub_settings SET company_name=$1 WHERE id=TRUE`, name)
	return err
}
