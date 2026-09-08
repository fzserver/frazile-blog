package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

type Tag struct {
	ID    int64
	Name  string
	Slug  string
	Count int
}

type Post struct {
	ID              int64
	Slug            string
	Title           string
	Summary         string
	BodyMD          string
	BodyHTML        template.HTML
	Cover           string
	AuthorID        int64
	Author          string // username
	AuthorName      string // display name or username
	AuthorAvatar    string
	Status          string
	PublishedAt     int64
	Created         int64
	Updated         int64
	Views           int
	Likes           int
	CommentCount    int
	CommentsEnabled bool
	Pinned          bool
	ReadingMin      int
	Tags            []Tag
	Liked           bool // for the current viewer
}

func (p *Post) IsPublished() bool {
	return p.Status == StatusPublished && p.PublishedAt <= time.Now().Unix()
}
func (p *Post) Scheduled() bool {
	return p.Status == StatusPublished && p.PublishedAt > time.Now().Unix()
}
func (p *Post) URL() string { return "/p/" + p.Slug }
func (p *Post) Date() time.Time {
	if p.PublishedAt > 0 {
		return time.Unix(p.PublishedAt, 0)
	}
	return time.Unix(p.Created, 0)
}

const postCols = `p.id, p.slug, p.title, p.summary, p.body_md, p.body_html, p.cover, p.author_id,
	u.username, COALESCE(NULLIF(u.display_name,''), u.username), u.avatar,
	p.status, p.published_at, p.created, p.updated, p.views, p.likes,
	(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.status='visible'),
	p.comments_enabled, p.pinned, p.reading_min`

const postFrom = ` FROM posts p JOIN users u ON u.id = p.author_id `

func scanPost(row interface{ Scan(...any) error }) (*Post, error) {
	var p Post
	var html string
	var ce, pin int
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.BodyMD, &html, &p.Cover, &p.AuthorID,
		&p.Author, &p.AuthorName, &p.AuthorAvatar,
		&p.Status, &p.PublishedAt, &p.Created, &p.Updated, &p.Views, &p.Likes, &p.CommentCount,
		&ce, &pin, &p.ReadingMin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.BodyHTML = template.HTML(html)
	p.CommentsEnabled, p.Pinned = ce == 1, pin == 1
	return &p, nil
}

// PostFilter selects posts for a listing.
type PostFilter struct {
	Status    string // "" = any; "published" means live (published and not scheduled)
	AuthorID  int64
	Tag       string // tag slug
	Query     string // full-text search
	Year      int
	Month     int
	Page      int
	PerPage   int
	Pinned    bool // pinned first (home page)
	SortViews bool
}

func (f PostFilter) where() (string, []any) {
	w := []string{"1=1"}
	var args []any
	switch f.Status {
	case StatusPublished:
		w = append(w, "p.status='published' AND p.published_at <= ?")
		args = append(args, now())
	case "":
	case "scheduled":
		w = append(w, "p.status='published' AND p.published_at > ?")
		args = append(args, now())
	default:
		w = append(w, "p.status = ?")
		args = append(args, f.Status)
	}
	if f.AuthorID > 0 {
		w = append(w, "p.author_id = ?")
		args = append(args, f.AuthorID)
	}
	if f.Tag != "" {
		w = append(w, "p.id IN (SELECT pt.post_id FROM post_tags pt JOIN tags t ON t.id = pt.tag_id WHERE t.slug = ?)")
		args = append(args, f.Tag)
	}
	if f.Query != "" {
		w = append(w, "p.id IN (SELECT rowid FROM posts_fts WHERE posts_fts MATCH ?)")
		args = append(args, ftsQuery(f.Query))
	}
	if f.Year > 0 {
		start := time.Date(f.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(1, 0, 0)
		if f.Month > 0 {
			start = time.Date(f.Year, time.Month(f.Month), 1, 0, 0, 0, 0, time.UTC)
			end = start.AddDate(0, 1, 0)
		}
		w = append(w, "p.published_at >= ? AND p.published_at < ?")
		args = append(args, start.Unix(), end.Unix())
	}
	return strings.Join(w, " AND "), args
}

// ftsQuery turns free text into a safe FTS5 query: every word quoted and
// prefix-matched, so punctuation in the input cannot break the parser.
func ftsQuery(q string) string {
	var parts []string
	for _, f := range strings.Fields(q) {
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		parts = append(parts, `"`+f+`"*`)
	}
	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, " ")
}

func (s *Store) ListPosts(ctx context.Context, f PostFilter) ([]Post, int, error) {
	if f.PerPage <= 0 {
		f.PerPage = 10
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	where, args := f.where()
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+postFrom+`WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "ORDER BY "
	if f.Pinned {
		order += "p.pinned DESC, "
	}
	if f.SortViews {
		order += "p.views DESC, "
	}
	if f.Status == StatusPublished || f.Status == "scheduled" {
		order += "p.published_at DESC"
	} else {
		order += "p.updated DESC"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+postCols+postFrom+`WHERE `+where+` `+order+` LIMIT ? OFFSET ?`,
		append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	s.attachTags(ctx, out)
	return out, total, nil
}

func (s *Store) attachTags(ctx context.Context, posts []Post) {
	if len(posts) == 0 {
		return
	}
	idx := map[int64]int{}
	ph := make([]string, len(posts))
	args := make([]any, len(posts))
	for i := range posts {
		idx[posts[i].ID] = i
		ph[i] = "?"
		args[i] = posts[i].ID
	}
	rows, err := s.db.QueryContext(ctx, `SELECT pt.post_id, t.id, t.name, t.slug FROM post_tags pt JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id IN (`+strings.Join(ph, ",")+`) ORDER BY t.name`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var pid int64
		var t Tag
		if rows.Scan(&pid, &t.ID, &t.Name, &t.Slug) == nil {
			i := idx[pid]
			posts[i].Tags = append(posts[i].Tags, t)
		}
	}
}

func (s *Store) PostBySlug(ctx context.Context, slug string) (*Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx, `SELECT `+postCols+postFrom+`WHERE p.slug = ?`, slug))
	if err != nil {
		return nil, err
	}
	ps := []Post{*p}
	s.attachTags(ctx, ps)
	return &ps[0], nil
}

func (s *Store) PostByID(ctx context.Context, id int64) (*Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx, `SELECT `+postCols+postFrom+`WHERE p.id = ?`, id))
	if err != nil {
		return nil, err
	}
	ps := []Post{*p}
	s.attachTags(ctx, ps)
	return &ps[0], nil
}

// SavePost inserts (ID == 0) or updates a post together with its tag list.
// The slug is made unique by suffixing -2, -3, ... when it collides.
func (s *Store) SavePost(ctx context.Context, p *Post, tags []string) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		t := now()
		base := p.Slug
		for i := 2; ; i++ {
			var other int64
			err := tx.QueryRowContext(ctx, `SELECT id FROM posts WHERE slug = ?`, p.Slug).Scan(&other)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && other == p.ID) {
				break
			}
			if err != nil {
				return err
			}
			p.Slug = fmt.Sprintf("%s-%d", base, i)
		}
		if p.ID == 0 {
			p.Created = t
			res, err := tx.ExecContext(ctx, `INSERT INTO posts(slug,title,summary,body_md,body_html,cover,author_id,status,published_at,created,updated,comments_enabled,pinned,reading_min)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				p.Slug, p.Title, p.Summary, p.BodyMD, string(p.BodyHTML), p.Cover, p.AuthorID, p.Status, p.PublishedAt, t, t, b2i(p.CommentsEnabled), b2i(p.Pinned), p.ReadingMin)
			if err != nil {
				return err
			}
			p.ID, _ = res.LastInsertId()
		} else {
			_, err := tx.ExecContext(ctx, `UPDATE posts SET slug=?,title=?,summary=?,body_md=?,body_html=?,cover=?,status=?,published_at=?,updated=?,comments_enabled=?,pinned=?,reading_min=? WHERE id=?`,
				p.Slug, p.Title, p.Summary, p.BodyMD, string(p.BodyHTML), p.Cover, p.Status, p.PublishedAt, t, b2i(p.CommentsEnabled), b2i(p.Pinned), p.ReadingMin, p.ID)
			if err != nil {
				return err
			}
		}
		p.Updated = t
		if _, err := tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id = ?`, p.ID); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, name := range tags {
			name = strings.TrimSpace(name)
			slug := Slugify(name)
			if name == "" || slug == "" || seen[slug] {
				continue
			}
			seen[slug] = true
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO tags(name, slug) VALUES(?,?)`, name, slug); err != nil {
				return err
			}
			var tid int64
			if err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE slug = ?`, slug).Scan(&tid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO post_tags(post_id, tag_id) VALUES(?,?)`, p.ID, tid); err != nil {
				return err
			}
		}
		// Tags no post uses any more disappear from the tag cloud.
		_, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM post_tags)`)
		return err
	})
}

func (s *Store) DeletePost(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM post_tags)`)
	return err
}

func (s *Store) IncView(ctx context.Context, id int64) {
	s.db.ExecContext(ctx, `UPDATE posts SET views = views + 1 WHERE id = ?`, id)
}

// ToggleLike flips the viewer's like and returns the new state and count.
func (s *Store) ToggleLike(ctx context.Context, postID, userID int64) (bool, int, error) {
	var liked bool
	var count int
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM post_likes WHERE post_id=? AND user_id=?`, postID, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO post_likes(post_id,user_id,created) VALUES(?,?,?)`, postID, userID, now()); err != nil {
				return err
			}
			liked = true
		}
		if _, err := tx.ExecContext(ctx, `UPDATE posts SET likes = (SELECT COUNT(*) FROM post_likes WHERE post_id=?) WHERE id=?`, postID, postID); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT likes FROM posts WHERE id=?`, postID).Scan(&count)
	})
	return liked, count, err
}

func (s *Store) UserLiked(ctx context.Context, postID, userID int64) bool {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_likes WHERE post_id=? AND user_id=?`, postID, userID).Scan(&n)
	return n > 0
}

// Tags returns every tag with its live post count, most used first.
func (s *Store) Tags(ctx context.Context) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.name, t.slug,
		(SELECT COUNT(*) FROM post_tags pt JOIN posts p ON p.id = pt.post_id WHERE pt.tag_id = t.id AND p.status='published' AND p.published_at <= ?) AS n
		FROM tags t ORDER BY n DESC, t.name`, now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Count); err != nil {
			return nil, err
		}
		if t.Count > 0 {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func (s *Store) TagBySlug(ctx context.Context, slug string) (*Tag, error) {
	var t Tag
	err := s.db.QueryRowContext(ctx, `SELECT id, name, slug FROM tags WHERE slug = ?`, slug).Scan(&t.ID, &t.Name, &t.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// Related picks live posts sharing the most tags with the given one.
func (s *Store) Related(ctx context.Context, postID int64, n int) ([]Post, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+postCols+postFrom+`
		WHERE p.id != ? AND p.status='published' AND p.published_at <= ?
		AND p.id IN (SELECT post_id FROM post_tags WHERE tag_id IN (SELECT tag_id FROM post_tags WHERE post_id = ?))
		ORDER BY (SELECT COUNT(*) FROM post_tags a WHERE a.post_id = p.id AND a.tag_id IN (SELECT tag_id FROM post_tags WHERE post_id = ?)) DESC, p.published_at DESC
		LIMIT ?`, postID, now(), postID, postID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	s.attachTags(ctx, out)
	return out, rows.Err()
}

type Month struct {
	Year  int
	Month int
	Count int
}

func (m Month) Name() string { return time.Month(m.Month).String() }

func (s *Store) Archive(ctx context.Context) ([]Month, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT strftime('%Y', published_at, 'unixepoch'), strftime('%m', published_at, 'unixepoch'), COUNT(*)
		FROM posts WHERE status='published' AND published_at <= ? GROUP BY 1,2 ORDER BY 1 DESC, 2 DESC`, now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Month
	for rows.Next() {
		var m Month
		if err := rows.Scan(&m.Year, &m.Month, &m.Count); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CountPosts(ctx context.Context, status string) int {
	var n int
	if status == "" {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts`).Scan(&n)
	} else {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts WHERE status = ?`, status).Scan(&n)
	}
	return n
}

// PostTitles maps slug -> title for the stats dashboard's "top posts" table.
func (s *Store) PostTitles(ctx context.Context, slugs []string) map[string]string {
	out := map[string]string{}
	if len(slugs) == 0 {
		return out
	}
	ph := make([]string, len(slugs))
	args := make([]any, len(slugs))
	for i, sl := range slugs {
		ph[i] = "?"
		args[i] = sl
	}
	rows, err := s.db.QueryContext(ctx, `SELECT slug, title FROM posts WHERE slug IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var sl, t string
		if rows.Scan(&sl, &t) == nil {
			out[sl] = t
		}
	}
	return out
}

// Slugify reduces a title to a URL slug: lowercase ASCII letters, digits and
// single dashes. Non-ASCII letters are dropped rather than transliterated.
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == ' ' || r == '-' || r == '_' || r == '.' || r == '/':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
	}
	return out
}
