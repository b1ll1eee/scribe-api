package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"scribe/backend/internal/domain/scribe"
	"scribe/backend/internal/ports"

	"github.com/lib/pq"
)

var _ ports.TagRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) CreateScribe(ctx context.Context, n *scribe.Scribe) error {
	query := `
		INSERT INTO scribes (owner_id, title, content, is_pinned, is_archived, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, n.OwnerID, n.Title, n.Content, n.IsPinned, n.IsArchived, now, now).
		Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create scribe: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateScribe(ctx context.Context, n *scribe.Scribe) error {
	query := `
		UPDATE scribes SET title = $1, content = $2, is_pinned = $3, is_archived = $4, updated_at = $5
		WHERE id = $6 AND owner_id = $7 AND deleted_at IS NULL
	`
	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, n.Title, n.Content, n.IsPinned, n.IsArchived, now, n.ID, n.OwnerID)
	if err != nil {
		return fmt.Errorf("update scribe: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update scribe rows affected: %w", err)
	}
	if rows == 0 {
		return errors.New("scribe not found or deleted")
	}
	n.UpdatedAt = now
	return nil
}

func (r *PostgresRepository) GetScribeByID(ctx context.Context, id string, ownerID string) (*scribe.Scribe, error) {
	query := `
		SELECT id, owner_id, title, content, is_pinned, is_archived, created_at, updated_at, deleted_at
		FROM scribes WHERE id = $1 AND owner_id = $2
	`
	var n scribe.Scribe
	err := r.db.QueryRowContext(ctx, query, id, ownerID).
		Scan(&n.ID, &n.OwnerID, &n.Title, &n.Content, &n.IsPinned, &n.IsArchived, &n.CreatedAt, &n.UpdatedAt, &n.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scribe not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get scribe by id: %w", err)
	}

	tags, err := r.getTagsForScribe(ctx, n.ID)
	if err != nil {
		return nil, fmt.Errorf("get tags for scribe: %w", err)
	}
	n.Tags = tags
	return &n, nil
}

func (r *PostgresRepository) DeleteScribe(ctx context.Context, id string, ownerID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM scribes WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete scribe: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("scribe not found")
	}
	return nil
}

func (r *PostgresRepository) SoftDeleteScribe(ctx context.Context, id string, ownerID string) error {
	query := `UPDATE scribes SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND owner_id = $3 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, time.Now(), id, ownerID)
	if err != nil {
		return fmt.Errorf("soft delete scribe: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("scribe already deleted or not found")
	}
	return nil
}

func (r *PostgresRepository) RestoreScribe(ctx context.Context, id string, ownerID string) error {
	query := `UPDATE scribes SET deleted_at = NULL, is_archived = false, updated_at = $1 WHERE id = $2 AND owner_id = $3`
	result, err := r.db.ExecContext(ctx, query, time.Now(), id, ownerID)
	if err != nil {
		return fmt.Errorf("restore scribe: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("scribe not found")
	}
	return nil
}

func (r *PostgresRepository) ArchiveScribe(ctx context.Context, id string, ownerID string) error {
	query := `UPDATE scribes SET is_archived = true, is_pinned = false, updated_at = $1 WHERE id = $2 AND owner_id = $3 AND deleted_at IS NULL AND is_archived = false`
	result, err := r.db.ExecContext(ctx, query, time.Now(), id, ownerID)
	if err != nil {
		return fmt.Errorf("archive scribe: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("scribe not found, already archived, or deleted")
	}
	return nil
}

func (r *PostgresRepository) TogglePinScribe(ctx context.Context, id string, ownerID string, pinned bool) error {
	query := `UPDATE scribes SET is_pinned = $1, updated_at = $2 WHERE id = $3 AND owner_id = $4 AND deleted_at IS NULL AND is_archived = false`
	result, err := r.db.ExecContext(ctx, query, pinned, time.Now(), id, ownerID)
	if err != nil {
		return fmt.Errorf("toggle pin scribe: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("scribe not found, archived, or deleted")
	}
	return nil
}

func (r *PostgresRepository) CountByOwner(ctx context.Context, ownerID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scribes WHERE owner_id = $1 AND deleted_at IS NULL`, ownerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count scribes: %w", err)
	}
	return count, nil
}

func (r *PostgresRepository) ListScribes(ctx context.Context, params ports.ScribeListParams) ([]*scribe.Scribe, int, error) {
	selectSQL := `SELECT DISTINCT n.id, n.owner_id, n.title, n.content, n.is_pinned, n.is_archived, n.created_at, n.updated_at, n.deleted_at FROM scribes n`
	countSQL := `SELECT COUNT(DISTINCT n.id) FROM scribes n`

	if len(params.Tags) > 0 {
		join := ` JOIN scribe_tags nt ON n.id = nt.scribe_id JOIN tags t ON nt.tag_id = t.id`
		selectSQL += join
		countSQL += join
	}

	whereClauses := []string{"n.owner_id = $1"}
	args := []interface{}{params.OwnerID}
	idx := 2

	if !params.IncludeDeleted {
		whereClauses = append(whereClauses, "n.deleted_at IS NULL")
	}

	if params.IsArchived != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("n.is_archived = $%d", idx))
		args = append(args, *params.IsArchived)
		idx++
	}

	if params.IsPinned != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("n.is_pinned = $%d", idx))
		args = append(args, *params.IsPinned)
		idx++
	}

	if strings.TrimSpace(params.Search) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(n.title ILIKE $%d OR n.content ILIKE $%d)", idx, idx))
		args = append(args, "%"+params.Search+"%")
		idx++
	}

	if len(params.Tags) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("t.name = ANY($%d)", idx))
		args = append(args, pq.Array(params.Tags))
		idx++
	}

	whereSQL := " WHERE " + strings.Join(whereClauses, " AND ")
	selectSQL += whereSQL
	countSQL += whereSQL

	sortBy := "n.created_at"
	if params.SortBy == "updated_at" || params.SortBy == "title" {
		sortBy = "n." + params.SortBy
	}
	sortOrder := "DESC"
	if strings.ToUpper(params.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	selectSQL += fmt.Sprintf(" ORDER BY n.is_pinned DESC, %s %s LIMIT $%d OFFSET $%d", sortBy, sortOrder, idx, idx+1)
	selectArgs := append(args, params.Limit, params.Offset)

	var total int
	err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count scribes: %w", err)
	}

	if total == 0 {
		return []*scribe.Scribe{}, 0, nil
	}

	rows, err := r.db.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list scribes: %w", err)
	}
	defer rows.Close()

	var scribes []*scribe.Scribe
	var scribeIDs []string

	for rows.Next() {
		var n scribe.Scribe
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.Content, &n.IsPinned, &n.IsArchived, &n.CreatedAt, &n.UpdatedAt, &n.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan scribe: %w", err)
		}
		scribes = append(scribes, &n)
		scribeIDs = append(scribeIDs, n.ID)
	}

	if len(scribes) > 0 {
		tagMap, err := r.batchGetTagsForScribes(ctx, scribeIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("batch get tags: %w", err)
		}
		for _, n := range scribes {
			if t, ok := tagMap[n.ID]; ok {
				n.Tags = t
			} else {
				n.Tags = []string{}
			}
		}
	}

	return scribes, total, nil
}

func (r *PostgresRepository) GetOrCreateMany(ctx context.Context, names []string) ([]*scribe.Tag, error) {
	if len(names) == 0 {
		return []*scribe.Tag{}, nil
	}

	uniqueMap := make(map[string]bool)
	var clean []string
	for _, n := range names {
		c := strings.ToLower(strings.TrimSpace(n))
		if c != "" && !uniqueMap[c] {
			uniqueMap[c] = true
			clean = append(clean, c)
		}
	}

	if len(clean) == 0 {
		return []*scribe.Tag{}, nil
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM tags WHERE name = ANY($1)`, pq.Array(clean))
	if err != nil {
		return nil, fmt.Errorf("query existing tags: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]*scribe.Tag)
	for rows.Next() {
		var t scribe.Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		existing[t.Name] = &t
	}

	var result []*scribe.Tag
	for _, name := range clean {
		if tag, ok := existing[name]; ok {
			result = append(result, tag)
			continue
		}
		var t scribe.Tag
		t.Name = name
		err := r.db.QueryRowContext(ctx, `INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, name).Scan(&t.ID)
		if err != nil {
			return nil, fmt.Errorf("upsert tag %s: %w", name, err)
		}
		result = append(result, &t)
	}

	return result, nil
}

func (r *PostgresRepository) AssociateWithScribe(ctx context.Context, scribeID string, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}
	for _, tagID := range tagIDs {
		_, err := r.db.ExecContext(ctx, `INSERT INTO scribe_tags (scribe_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, scribeID, tagID)
		if err != nil {
			return fmt.Errorf("associate tag with scribe: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) ClearScribeTags(ctx context.Context, scribeID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scribe_tags WHERE scribe_id = $1`, scribeID)
	if err != nil {
		return fmt.Errorf("clear scribe tags: %w", err)
	}
	return nil
}

func (r *PostgresRepository) getTagsForScribe(ctx context.Context, scribeID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.name FROM tags t JOIN scribe_tags nt ON t.id = nt.tag_id WHERE nt.scribe_id = $1`, scribeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (r *PostgresRepository) batchGetTagsForScribes(ctx context.Context, scribeIDs []string) (map[string][]string, error) {
	tagMap := make(map[string][]string)
	if len(scribeIDs) == 0 {
		return tagMap, nil
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT nt.scribe_id, t.name FROM scribe_tags nt JOIN tags t ON nt.tag_id = t.id WHERE nt.scribe_id = ANY($1)`,
		pq.Array(scribeIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var scribeID, tagName string
		if err := rows.Scan(&scribeID, &tagName); err != nil {
			return nil, err
		}
		tagMap[scribeID] = append(tagMap[scribeID], tagName)
	}
	return tagMap, nil
}
