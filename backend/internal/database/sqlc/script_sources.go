package sqlc

import (
	"context"
	"encoding/json"
)

func (q *Queries) ImportScript(ctx context.Context, p UpsertAppParams, files []byte) (int64, error) {
	var id int64
	err := q.db.QueryRow(ctx, `WITH app AS (
		INSERT INTO apps (slug,name,summary,kind,language,area,area_id,folder_path,owner_id,manifest,created_by)
		VALUES ($1,$2,$3,'script',$4,$5,$6,$7,$8,$9,$8) RETURNING id
	), source AS (
		INSERT INTO script_sources(app_id,files) SELECT id,$10 FROM app RETURNING app_id
	) SELECT app_id FROM source`, p.Slug, p.Name, p.Summary, p.Language, p.Area, p.AreaID, p.FolderPath, p.OwnerID, p.Manifest, files).Scan(&id)
	return id, err
}

func (q *Queries) ScriptSources(ctx context.Context, id int64) (json.RawMessage, error) {
	var raw json.RawMessage
	err := q.db.QueryRow(ctx, `SELECT files FROM script_sources WHERE app_id=$1`, id).Scan(&raw)
	return raw, err
}
