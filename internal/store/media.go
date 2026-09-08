package store

import (
	"context"
	"database/sql"
	"errors"
)

type Media struct {
	ID           int64
	Name         string // path relative to the media dir, e.g. 2026/09/ab12.webp
	OriginalName string
	Mime         string
	Kind         string // image | video
	Size         int64
	Width        int
	Height       int
	UploaderID   int64
	Created      int64
	Uploader     string
}

func (m *Media) URL() string { return "/media/" + m.Name }

func (s *Store) AddMedia(ctx context.Context, m *Media) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO media(name,original_name,mime,kind,size,width,height,uploader_id,created) VALUES(?,?,?,?,?,?,?,?,?)`,
		m.Name, m.OriginalName, m.Mime, m.Kind, m.Size, m.Width, m.Height, m.UploaderID, now())
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) MediaByID(ctx context.Context, id int64) (*Media, error) {
	var m Media
	err := s.db.QueryRowContext(ctx, `SELECT m.id,m.name,m.original_name,m.mime,m.kind,m.size,m.width,m.height,m.uploader_id,m.created, COALESCE(u.username,'')
		FROM media m LEFT JOIN users u ON u.id = m.uploader_id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.Name, &m.OriginalName, &m.Mime, &m.Kind, &m.Size, &m.Width, &m.Height, &m.UploaderID, &m.Created, &m.Uploader)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (s *Store) ListMedia(ctx context.Context, kind string, page, per int) ([]Media, int, error) {
	where, args := "1=1", []any{}
	if kind != "" {
		where, args = "m.kind = ?", []any{kind}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media m WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.name,m.original_name,m.mime,m.kind,m.size,m.width,m.height,m.uploader_id,m.created, COALESCE(u.username,'')
		FROM media m LEFT JOIN users u ON u.id = m.uploader_id WHERE `+where+` ORDER BY m.created DESC LIMIT ? OFFSET ?`,
		append(args, per, (page-1)*per)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Media
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.ID, &m.Name, &m.OriginalName, &m.Mime, &m.Kind, &m.Size, &m.Width, &m.Height, &m.UploaderID, &m.Created, &m.Uploader); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func (s *Store) DeleteMedia(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM media WHERE id = ?`, id)
	return err
}

// MediaTotals reports count and bytes for the admin dashboard.
func (s *Store) MediaTotals(ctx context.Context) (int, int64) {
	var n int
	var size int64
	s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(size),0) FROM media`).Scan(&n, &size)
	return n, size
}
