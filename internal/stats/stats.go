// Package stats records page views to their own SQLite file and answers the
// admin dashboard's questions about them. Recording is asynchronous: handlers
// push a Visit onto a channel and one writer goroutine batch-inserts, so the
// request path never waits on disk. Rows are also appended to a daily JSONL
// journal so the history survives a lost database.
package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fzserver/frazile-blog/internal/geoip"
	_ "modernc.org/sqlite"
)

type Visit struct {
	TS       int64  `json:"ts"`
	IP       string `json:"ip"`
	UA       string `json:"ua"`
	Browser  string `json:"browser"`
	OS       string `json:"os"`
	Device   string `json:"device"`
	UserID   int64  `json:"user_id"`
	User     string `json:"user"`
	Path     string `json:"path"`
	PostID   int64  `json:"post_id"`
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	Provider string `json:"provider"`
	VPN      bool   `json:"vpn"`
	Attribution
	Landing bool `json:"landing"`
}

type Store struct {
	db         *sql.DB
	vpn        *vpnRanges
	geo        *geoip.Resolver
	ch         chan Visit
	log        *slog.Logger
	done       chan struct{}
	journalDir string
	journal    *os.File
	journalDay string
}

func Open(path, vpnFile, journalDir string, geo *geoip.Resolver, log *slog.Logger) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS visits(
		id       INTEGER PRIMARY KEY,
		ts       INTEGER NOT NULL,
		ip       TEXT NOT NULL DEFAULT '',
		ua       TEXT NOT NULL DEFAULT '',
		browser  TEXT NOT NULL DEFAULT '',
		os       TEXT NOT NULL DEFAULT '',
		device   TEXT NOT NULL DEFAULT '',
		user_id  INTEGER NOT NULL DEFAULT 0,
		username TEXT NOT NULL DEFAULT '',
		path     TEXT NOT NULL DEFAULT '',
		post_id  INTEGER NOT NULL DEFAULT 0,
		country  TEXT NOT NULL DEFAULT '',
		region   TEXT NOT NULL DEFAULT '',
		city     TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		vpn      INTEGER NOT NULL DEFAULT 0,
		referrer TEXT NOT NULL DEFAULT '',
		source   TEXT NOT NULL DEFAULT '',
		medium   TEXT NOT NULL DEFAULT '',
		campaign TEXT NOT NULL DEFAULT '',
		landing  INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS visits_ts ON visits(ts);
	CREATE INDEX IF NOT EXISTS visits_ip ON visits(ip);
	CREATE INDEX IF NOT EXISTS visits_post ON visits(post_id, ts);
	CREATE INDEX IF NOT EXISTS visits_landing ON visits(landing, ts);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("stats schema: %w", err)
	}
	if journalDir != "" {
		if err := os.MkdirAll(journalDir, 0o755); err != nil {
			db.Close()
			return nil, fmt.Errorf("stats journal dir: %w", err)
		}
	}
	s := &Store{db: db, vpn: loadVPNRanges(vpnFile), geo: geo, ch: make(chan Visit, 4096), log: log, done: make(chan struct{}), journalDir: journalDir}
	go s.writer()
	return s, nil
}

func (s *Store) GeoEnabled() bool { return s.geo != nil && s.geo.Enabled() }

// Record classifies and enqueues a visit. It never blocks: on a full buffer
// the visit is dropped rather than slowing the request.
func (s *Store) Record(v Visit) {
	v.Browser, v.OS, v.Device = classify(v.UA)
	v.VPN = s.vpn.contains(v.IP)
	if s.geo != nil {
		loc := s.geo.Lookup(v.IP)
		v.Country, v.Region, v.City, v.Provider = loc.Country, loc.Region, loc.City, loc.Provider
	}
	v.UA = truncate(v.UA, 512)
	v.Path = truncate(v.Path, 512)
	select {
	case s.ch <- v:
	default:
	}
}

func (s *Store) writer() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	batch := make([]Visit, 0, 256)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		tx, err := s.db.Begin()
		if err != nil {
			s.log.Error("stats tx", "err", err)
			batch = batch[:0]
			return
		}
		stmt, err := tx.Prepare(`INSERT INTO visits(ts,ip,ua,browser,os,device,user_id,username,path,post_id,country,region,city,provider,vpn,referrer,source,medium,campaign,landing)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			tx.Rollback()
			batch = batch[:0]
			return
		}
		for _, v := range batch {
			stmt.Exec(v.TS, v.IP, v.UA, v.Browser, v.OS, v.Device, v.UserID, v.User, v.Path, v.PostID,
				v.Country, v.Region, v.City, v.Provider, b2i(v.VPN), v.Referrer, v.Source, v.Medium, v.Campaign, b2i(v.Landing))
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			s.log.Error("stats commit", "err", err)
		}
		s.appendJournal(batch)
		batch = batch[:0]
	}
	for {
		select {
		case v := <-s.ch:
			batch = append(batch, v)
			if len(batch) >= 256 {
				flush()
			}
		case <-tick.C:
			flush()
		case <-s.done:
			for {
				select {
				case v := <-s.ch:
					batch = append(batch, v)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (s *Store) appendJournal(batch []Visit) {
	if s.journalDir == "" || len(batch) == 0 {
		return
	}
	day := time.Unix(batch[0].TS, 0).UTC().Format("2006-01-02")
	if s.journal == nil || day != s.journalDay {
		if s.journal != nil {
			s.journal.Close()
		}
		f, err := os.OpenFile(filepath.Join(s.journalDir, "visits-"+day+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			s.log.Error("stats journal open", "err", err)
			s.journal = nil
			return
		}
		s.journal, s.journalDay = f, day
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, v := range batch {
		enc.Encode(v)
	}
	if _, err := s.journal.WriteString(b.String()); err != nil {
		s.log.Error("stats journal write", "err", err)
	}
}

func (s *Store) Close() error {
	close(s.done)
	time.Sleep(50 * time.Millisecond)
	if s.journal != nil {
		s.journal.Close()
	}
	return s.db.Close()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- queries ----

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Point struct {
	Label   string `json:"label"`
	Views   int    `json:"views"`
	Uniques int    `json:"uniques"`
}

// Summary is one dashboard load: everything for a period in a single struct.
type Summary struct {
	Period    string       `json:"period"`
	Views     int          `json:"views"`
	Uniques   int          `json:"uniques"`
	Landings  int          `json:"landings"`
	Bots      int          `json:"bots"`
	Series    []Point      `json:"series"`
	TopPaths  []NamedCount `json:"top_paths"`
	TopPosts  []NamedCount `json:"top_posts"` // by post slug (path)
	Referrers []NamedCount `json:"referrers"`
	Sources   []NamedCount `json:"sources"`
	Mediums   []NamedCount `json:"mediums"`
	Countries []NamedCount `json:"countries"`
	Cities    []NamedCount `json:"cities"`
	Browsers  []NamedCount `json:"browsers"`
	OSes      []NamedCount `json:"oses"`
	Devices   []NamedCount `json:"devices"`
	Users     []NamedCount `json:"users"`
	Hours     []int        `json:"hours"` // views by hour of day (UTC), 24 buckets
}

// humans excludes crawlers and scripts from every headline number.
const humans = ` AND device != 'bot' `

func (s *Store) Summary(ctx context.Context, period string) (*Summary, error) {
	t := time.Now()
	var since int64
	var buckets int
	var width time.Duration
	var label string
	switch period {
	case "24h":
		since, buckets, width, label = t.Add(-24*time.Hour).Unix(), 24, time.Hour, "15:04"
	case "7d":
		since, buckets, width, label = t.Add(-7*24*time.Hour).Unix(), 7, 24*time.Hour, "Jan 2"
	case "90d":
		since, buckets, width, label = t.Add(-90*24*time.Hour).Unix(), 90, 24*time.Hour, "Jan 2"
	case "all":
		since, buckets, width, label = 0, 12, 30*24*time.Hour, "Jan 06"
	default:
		period = "30d"
		since, buckets, width, label = t.Add(-30*24*time.Hour).Unix(), 30, 24*time.Hour, "Jan 2"
	}
	sm := &Summary{Period: period}
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits WHERE ts >= ?`+humans, since).Scan(&sm.Views)
	s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT ip) FROM visits WHERE ts >= ?`+humans, since).Scan(&sm.Uniques)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits WHERE ts >= ? AND landing=1`+humans, since).Scan(&sm.Landings)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits WHERE ts >= ? AND device='bot'`, since).Scan(&sm.Bots)

	// Series: fixed buckets ending now, so a quiet day still shows as zero.
	end := t
	if width >= 24*time.Hour {
		end = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Add(24 * time.Hour)
	} else {
		end = t.Truncate(time.Hour).Add(time.Hour)
	}
	for i := buckets - 1; i >= 0; i-- {
		hi := end.Add(-time.Duration(i) * width)
		lo := hi.Add(-width)
		p := Point{Label: lo.Format(label)}
		s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(DISTINCT ip) FROM visits WHERE ts >= ? AND ts < ?`+humans, lo.Unix(), hi.Unix()).Scan(&p.Views, &p.Uniques)
		sm.Series = append(sm.Series, p)
	}
	sm.TopPaths, _ = s.topBy(ctx, "path", since, 15, "")
	sm.TopPosts, _ = s.topBy(ctx, "path", since, 10, " AND post_id > 0 ")
	sm.Referrers, _ = s.topBy(ctx, "referrer", since, 10, " AND landing=1 AND referrer != '' ")
	sm.Sources, _ = s.topBy(ctx, "source", since, 10, " AND landing=1 ")
	sm.Mediums, _ = s.topBy(ctx, "medium", since, 6, " AND landing=1 ")
	sm.Countries, _ = s.topBy(ctx, "country", since, 12, " AND country != '' ")
	sm.Cities, _ = s.topBy(ctx, "city", since, 10, " AND city != '' ")
	sm.Browsers, _ = s.topBy(ctx, "browser", since, 8, "")
	sm.OSes, _ = s.topBy(ctx, "os", since, 8, "")
	sm.Devices, _ = s.topBy(ctx, "device", since, 4, "")
	sm.Users, _ = s.topBy(ctx, "username", since, 10, " AND user_id > 0 ")
	sm.Hours = make([]int, 24)
	rows, err := s.db.QueryContext(ctx, `SELECT CAST(strftime('%H', ts, 'unixepoch') AS INTEGER), COUNT(*) FROM visits WHERE ts >= ?`+humans+` GROUP BY 1`, since)
	if err == nil {
		for rows.Next() {
			var h, n int
			if rows.Scan(&h, &n) == nil && h >= 0 && h < 24 {
				sm.Hours[h] = n
			}
		}
		rows.Close()
	}
	return sm, nil
}

func (s *Store) topBy(ctx context.Context, col string, since int64, limit int, extra string) ([]NamedCount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+col+`, COUNT(*) AS n FROM visits WHERE ts >= ?`+humans+extra+` GROUP BY `+col+` ORDER BY n DESC LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NamedCount
	for rows.Next() {
		var nc NamedCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

// Query filters the visit log.
type Query struct {
	IP, Path, Country, User, Device, Source string
	Since                                   int64
	Bots                                    bool // include bots
	Page, PerPage                           int
}

type Row struct {
	ID       int64  `json:"id"`
	TS       int64  `json:"ts"`
	IP       string `json:"ip"`
	UA       string `json:"ua"`
	Browser  string `json:"browser"`
	OS       string `json:"os"`
	Device   string `json:"device"`
	User     string `json:"user"`
	Path     string `json:"path"`
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	Provider string `json:"provider"`
	VPN      bool   `json:"vpn"`
	Referrer string `json:"referrer"`
	Source   string `json:"source"`
	Medium   string `json:"medium"`
	Landing  bool   `json:"landing"`
}

func (r Row) Time() time.Time { return time.Unix(r.TS, 0) }

func (q Query) where() (string, []any) {
	w := []string{"1=1"}
	var args []any
	add := func(col, val string) {
		if val != "" {
			w = append(w, col+" LIKE ?")
			args = append(args, "%"+val+"%")
		}
	}
	if q.IP != "" {
		w = append(w, "ip = ?")
		args = append(args, q.IP)
	}
	add("path", q.Path)
	add("country", q.Country)
	add("username", q.User)
	add("source", q.Source)
	if q.Device != "" {
		w = append(w, "device = ?")
		args = append(args, q.Device)
	} else if !q.Bots {
		w = append(w, "device != 'bot'")
	}
	if q.Since > 0 {
		w = append(w, "ts >= ?")
		args = append(args, q.Since)
	}
	return strings.Join(w, " AND "), args
}

func (s *Store) Search(ctx context.Context, q Query) ([]Row, int, error) {
	if q.PerPage <= 0 {
		q.PerPage = 50
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	where, args := q.where()
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,ts,ip,ua,browser,os,device,username,path,country,region,city,provider,vpn,referrer,source,medium,landing
		FROM visits WHERE `+where+` ORDER BY ts DESC LIMIT ? OFFSET ?`, append(args, q.PerPage, (q.Page-1)*q.PerPage)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var r Row
		var vpn, landing int
		if err := rows.Scan(&r.ID, &r.TS, &r.IP, &r.UA, &r.Browser, &r.OS, &r.Device, &r.User, &r.Path, &r.Country, &r.Region, &r.City, &r.Provider, &vpn, &r.Referrer, &r.Source, &r.Medium, &landing); err != nil {
			return nil, 0, err
		}
		r.VPN, r.Landing = vpn == 1, landing == 1
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// PostViews is the per-post table for the admin posts list: views in the
// period keyed by post id.
func (s *Store) PostViews(ctx context.Context, since int64) map[int64]int {
	out := map[int64]int{}
	rows, err := s.db.QueryContext(ctx, `SELECT post_id, COUNT(*) FROM visits WHERE post_id > 0 AND ts >= ?`+humans+` GROUP BY post_id`, since)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if rows.Scan(&id, &n) == nil {
			out[id] = n
		}
	}
	return out
}

func (s *Store) Total(ctx context.Context) int {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits`).Scan(&n)
	return n
}
