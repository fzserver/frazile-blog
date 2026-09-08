package store

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	RoleReader = "reader"
	RoleAuthor = "author"
	RoleAdmin  = "admin"
)

type User struct {
	ID          int64
	Username    string
	Email       string
	DisplayName string
	Bio         string
	Website     string
	Avatar      string // URL path, "" = none
	Role        string
	Verified    bool
	Banned      bool
	Created     int64
	LastSeen    int64

	// Filled by profile queries only.
	PostCount    int
	CommentCount int
}

func (u *User) IsAdmin() bool  { return u != nil && u.Role == RoleAdmin }
func (u *User) CanWrite() bool { return u != nil && (u.Role == RoleAdmin || u.Role == RoleAuthor) }
func (u *User) Name() string {
	if u == nil {
		return ""
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

const userCols = `id, username, email, display_name, bio, website, avatar, role, verified, banned, created, last_seen`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var v, b int
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Bio, &u.Website, &u.Avatar, &u.Role, &v, &b, &u.Created, &u.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Verified, u.Banned = v == 1, b == 1
	return &u, nil
}

func hashPassword(password string, salt []byte, iters int) []byte {
	key, err := pbkdf2.Key(sha256.New, password, salt, iters, 32)
	if err != nil {
		panic(err)
	}
	return key
}

// CreateUser inserts an unverified account. Registration is a two-step flow:
// the row exists so the username is reserved, but Login refuses it until
// MarkVerified has run (after the e-mailed code checks out).
func (s *Store) CreateUser(ctx context.Context, username, email, password, role string, verified bool) (*User, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	hash := hashPassword(password, salt, s.Iters)
	email = strings.ToLower(strings.TrimSpace(email))
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(username, email, role, verified, salt, hash, iters, created) VALUES(?,?,?,?,?,?,?,?)`,
		username, email, role, b2i(verified), salt, hash, s.Iters, now())
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrTaken
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.UserByID(ctx, id)
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *Store) UserByUsername(ctx context.Context, name string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, name))
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email = ?`, strings.TrimSpace(email)))
}

// UserByLogin accepts a username or an e-mail address.
func (s *Store) UserByLogin(ctx context.Context, login string) (*User, error) {
	login = strings.TrimSpace(login)
	if strings.Contains(login, "@") {
		return s.UserByEmail(ctx, login)
	}
	return s.UserByUsername(ctx, login)
}

// CheckPassword verifies credentials. An unknown login still burns a hash so
// timing does not reveal whether the account exists.
func (s *Store) CheckPassword(ctx context.Context, login, password string) (*User, error) {
	u, err := s.UserByLogin(ctx, login)
	if errors.Is(err, ErrNotFound) {
		hashPassword(password, []byte("decoy-salt-16by."), s.Iters)
		return nil, ErrBadCredentials
	}
	if err != nil {
		return nil, err
	}
	var salt, hash []byte
	var iters int
	if err := s.db.QueryRowContext(ctx, `SELECT salt, hash, iters FROM users WHERE id = ?`, u.ID).Scan(&salt, &hash, &iters); err != nil {
		return nil, err
	}
	if !hmac.Equal(hash, hashPassword(password, salt, iters)) {
		return nil, ErrBadCredentials
	}
	return u, nil
}

func (s *Store) SetPassword(ctx context.Context, id int64, password string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET salt=?, hash=?, iters=? WHERE id=?`, salt, hashPassword(password, salt, s.Iters), s.Iters, id)
	return err
}

func (s *Store) MarkVerified(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET verified=1 WHERE id=?`, id)
	return err
}

func (s *Store) UpdateProfile(ctx context.Context, id int64, display, bio, website string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET display_name=?, bio=?, website=? WHERE id=?`, display, bio, website, id)
	return err
}

func (s *Store) SetAvatar(ctx context.Context, id int64, path string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET avatar=? WHERE id=?`, path, id)
	return err
}

func (s *Store) SetEmail(ctx context.Context, id int64, email string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET email=? WHERE id=?`, strings.ToLower(strings.TrimSpace(email)), id)
	if isUniqueErr(err) {
		return ErrTaken
	}
	return err
}

func (s *Store) SetRole(ctx context.Context, id int64, role string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) SetBanned(ctx context.Context, id int64, banned bool) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET banned=? WHERE id=?`, b2i(banned), id); err != nil {
		return err
	}
	if banned {
		return s.DeleteUserSessions(ctx, id)
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) TouchLastSeen(ctx context.Context, id int64) {
	s.db.ExecContext(ctx, `UPDATE users SET last_seen=? WHERE id=? AND last_seen < ?`, now(), id, now()-300)
}

func (s *Store) AdminExists(ctx context.Context) bool {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&n)
	return n > 0
}

func (s *Store) CountUsers(ctx context.Context, since int64) int {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE verified=1 AND created >= ?`, since).Scan(&n)
	return n
}

// ListUsers pages through accounts for the admin panel; q matches username,
// e-mail or display name.
func (s *Store) ListUsers(ctx context.Context, q, role string, page, per int) ([]User, int, error) {
	where, args := "1=1", []any{}
	if q != "" {
		like := "%" + q + "%"
		where += " AND (username LIKE ? OR email LIKE ? OR display_name LIKE ?)"
		args = append(args, like, like, like)
	}
	if role != "" {
		where += " AND role = ?"
		args = append(args, role)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE `+where+` ORDER BY created DESC LIMIT ? OFFSET ?`,
		append(args, per, (page-1)*per)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	return out, total, rows.Err()
}

// PurgeUnverified frees usernames claimed by registrations that never
// finished: rows older than the cutoff that were never verified.
func (s *Store) PurgeUnverified(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE verified=0 AND created < ?`, time.Now().Add(-olderThan).Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ProfileCounts fills PostCount/CommentCount.
func (s *Store) ProfileCounts(ctx context.Context, u *User) {
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts WHERE author_id=? AND status='published' AND published_at <= ?`, u.ID, now()).Scan(&u.PostCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE user_id=? AND status='visible'`, u.ID).Scan(&u.CommentCount)
}

// ---- sessions ----

const sessionTTL = 90 * 24 * time.Hour

func (s *Store) CreateSession(ctx context.Context, userID int64, ua, ip string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	th := sha256.Sum256([]byte(token))
	t := time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, created, expires, ua, ip) VALUES(?,?,?,?,?,?)`,
		th[:], userID, t.Unix(), t.Add(sessionTTL).Unix(), truncate(ua, 256), ip)
	return token, err
}

func (s *Store) UserBySession(ctx context.Context, token string) (*User, error) {
	th := sha256.Sum256([]byte(token))
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.email, u.display_name, u.bio, u.website, u.avatar, u.role, u.verified, u.banned, u.created, u.last_seen
		 FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ? AND s.expires > ?`, th[:], now()))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoSession
	}
	return u, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	th := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, th[:])
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) PurgeSessions(ctx context.Context) {
	s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires < ?`, now())
	s.db.ExecContext(ctx, `DELETE FROM email_codes WHERE expires < ?`, now()-3600)
}

// ---- e-mail codes ----

var (
	ErrCodeThrottled = errors.New("code sent too recently")
	ErrBadCode       = errors.New("wrong code")
	ErrCodeExpired   = errors.New("code expired")
)

const (
	codeTTL       = 10 * time.Minute
	codeResend    = 60 * time.Second
	codeAttempts  = 5
	PurposeVerify = "verify"
	PurposeReset  = "reset"
	PurposeEmail  = "email"
)

func normEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// IssueCode mints a six-digit code for the address and purpose; only its hash
// is stored. A code issued under a minute ago is not replaced.
func (s *Store) IssueCode(ctx context.Context, email, purpose string) (string, error) {
	email = normEmail(email)
	t := time.Now()
	var sent int64
	err := s.db.QueryRowContext(ctx, `SELECT sent FROM email_codes WHERE email = ? AND purpose = ?`, email, purpose).Scan(&sent)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && t.Unix()-sent < int64(codeResend/time.Second) {
		return "", ErrCodeThrottled
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	h := sha256.Sum256([]byte(code))
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO email_codes(email, purpose, hash, expires, attempts, sent) VALUES(?,?,?,?,0,?)
		 ON CONFLICT(email, purpose) DO UPDATE SET hash=excluded.hash, expires=excluded.expires, attempts=0, sent=excluded.sent`,
		email, purpose, h[:], t.Add(codeTTL).Unix(), t.Unix())
	return code, err
}

// CheckCode verifies without consuming; each wrong guess counts toward the cap.
func (s *Store) CheckCode(ctx context.Context, email, purpose, code string) error {
	email = normEmail(email)
	var hash []byte
	var expires int64
	var attempts int
	err := s.db.QueryRowContext(ctx, `SELECT hash, expires, attempts FROM email_codes WHERE email = ? AND purpose = ?`, email, purpose).
		Scan(&hash, &expires, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCodeExpired
	}
	if err != nil {
		return err
	}
	if now() > expires || attempts >= codeAttempts {
		return ErrCodeExpired
	}
	h := sha256.Sum256([]byte(strings.TrimSpace(code)))
	if !hmac.Equal(hash, h[:]) {
		s.db.ExecContext(ctx, `UPDATE email_codes SET attempts = attempts + 1 WHERE email = ? AND purpose = ?`, email, purpose)
		return ErrBadCode
	}
	return nil
}

func (s *Store) ConsumeCode(ctx context.Context, email, purpose string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM email_codes WHERE email = ? AND purpose = ?`, normEmail(email), purpose)
	return err
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
