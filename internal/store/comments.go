package store

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"strings"
)

const (
	CommentVisible = "visible"
	CommentPending = "pending"
	CommentHidden  = "hidden"
)

type Comment struct {
	ID       int64
	PostID   int64
	UserID   int64
	ParentID int64
	Body     string
	BodyHTML template.HTML
	Status   string
	Created  int64
	Edited   int64

	Username    string
	DisplayName string
	Avatar      string
	Role        string
	PostTitle   string
	PostSlug    string
	Replies     []*Comment
}

func (c *Comment) Name() string {
	if c.DisplayName != "" {
		return c.DisplayName
	}
	return c.Username
}

const commentCols = `c.id, c.post_id, c.user_id, c.parent_id, c.body, c.status, c.created, c.edited,
	u.username, u.display_name, u.avatar, u.role, p.title, p.slug`
const commentFrom = ` FROM comments c JOIN users u ON u.id = c.user_id JOIN posts p ON p.id = c.post_id `

func scanComment(row interface{ Scan(...any) error }) (*Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.PostID, &c.UserID, &c.ParentID, &c.Body, &c.Status, &c.Created, &c.Edited,
		&c.Username, &c.DisplayName, &c.Avatar, &c.Role, &c.PostTitle, &c.PostSlug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Comments are plain text: escape, then turn newlines into <br>. No
	// markdown, no HTML, so a commenter can never inject markup.
	c.BodyHTML = template.HTML(strings.ReplaceAll(template.HTMLEscapeString(c.Body), "\n", "<br>"))
	return &c, nil
}

func (s *Store) AddComment(ctx context.Context, postID, userID, parentID int64, body, status string) (int64, error) {
	if parentID > 0 {
		// Replies stay one level deep: replying to a reply attaches to its parent.
		var pp, ppost int64
		err := s.db.QueryRowContext(ctx, `SELECT parent_id, post_id FROM comments WHERE id = ?`, parentID).Scan(&pp, &ppost)
		if err != nil || ppost != postID {
			return 0, ErrNotFound
		}
		if pp > 0 {
			parentID = pp
		}
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO comments(post_id,user_id,parent_id,body,status,created) VALUES(?,?,?,?,?,?)`,
		postID, userID, parentID, body, status, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CommentByID(ctx context.Context, id int64) (*Comment, error) {
	return scanComment(s.db.QueryRowContext(ctx, `SELECT `+commentCols+commentFrom+`WHERE c.id = ?`, id))
}

// CommentsForPost returns the thread tree. viewerID's own pending comments
// are included so a commenter sees what they just posted while it awaits
// moderation; hidden ones are shown only to admins.
func (s *Store) CommentsForPost(ctx context.Context, postID, viewerID int64, admin bool) ([]*Comment, int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+commentCols+commentFrom+`WHERE c.post_id = ? ORDER BY c.created`, postID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	byID := map[int64]*Comment{}
	var order []*Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}
		if c.Status != CommentVisible && !admin && c.UserID != viewerID {
			continue
		}
		byID[c.ID] = c
		order = append(order, c)
	}
	var roots []*Comment
	n := 0
	for _, c := range order {
		n++
		if c.ParentID == 0 {
			roots = append(roots, c)
		} else if p, ok := byID[c.ParentID]; ok {
			p.Replies = append(p.Replies, c)
		} else {
			roots = append(roots, c)
		}
	}
	return roots, n, rows.Err()
}

func (s *Store) ListComments(ctx context.Context, status string, page, per int) ([]Comment, int, error) {
	where, args := "1=1", []any{}
	if status != "" {
		where, args = "c.status = ?", []any{status}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+commentFrom+`WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+commentCols+commentFrom+`WHERE `+where+` ORDER BY c.created DESC LIMIT ? OFFSET ?`,
		append(args, per, (page-1)*per)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	return out, total, rows.Err()
}

func (s *Store) UserComments(ctx context.Context, userID int64, n int) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+commentCols+commentFrom+`WHERE c.user_id = ? AND c.status='visible' AND p.status='published' ORDER BY c.created DESC LIMIT ?`, userID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) SetCommentStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE comments SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) EditComment(ctx context.Context, id int64, body string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE comments SET body=?, edited=? WHERE id=?`, body, now(), id)
	return err
}

// DeleteComment removes a comment and any replies to it.
func (s *Store) DeleteComment(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE parent_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, id)
		return err
	})
}

func (s *Store) CountComments(ctx context.Context, status string) int {
	var n int
	if status == "" {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments`).Scan(&n)
	} else {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE status=?`, status).Scan(&n)
	}
	return n
}

// RecentCommentCountByUser is the flood-control input: comments posted by
// the user in the last window.
func (s *Store) RecentCommentCountByUser(ctx context.Context, userID int64, window int64) int {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE user_id=? AND created > ?`, userID, now()-window).Scan(&n)
	return n
}
