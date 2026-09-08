package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Iters = 1000
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUsersAndSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "alice", "Alice@Example.com", "password1", RoleReader, false)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "alice@example.com" || u.Verified {
		t.Fatalf("user = %+v", u)
	}
	if _, err := s.CreateUser(ctx, "ALICE", "x@example.com", "password1", RoleReader, false); !errors.Is(err, ErrTaken) {
		t.Fatalf("case-insensitive username should be taken, got %v", err)
	}
	if _, err := s.CheckPassword(ctx, "alice", "wrong"); !errors.Is(err, ErrBadCredentials) {
		t.Fatal("wrong password accepted")
	}
	if _, err := s.CheckPassword(ctx, "nobody", "x"); !errors.Is(err, ErrBadCredentials) {
		t.Fatal("unknown user")
	}
	if got, err := s.CheckPassword(ctx, "alice@example.com", "password1"); err != nil || got.ID != u.ID {
		t.Fatalf("login by email: %v", err)
	}
	tok, err := s.CreateSession(ctx, u.ID, "ua", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.UserBySession(ctx, tok); err != nil || got.ID != u.ID {
		t.Fatalf("session lookup: %v", err)
	}
	s.DeleteSession(ctx, tok)
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, ErrNoSession) {
		t.Fatal("session survived delete")
	}
	if n, _ := s.PurgeUnverified(ctx, 0); n != 0 {
		t.Fatal("purge should skip fresh rows")
	}
	if n, _ := s.PurgeUnverified(ctx, -time.Hour); n != 1 {
		t.Fatalf("purge unverified = %d", n)
	}
}

func TestCodes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	code, err := s.IssueCode(ctx, "Bob@example.com", PurposeVerify)
	if err != nil || len(code) != 6 {
		t.Fatalf("issue: %q %v", code, err)
	}
	if _, err := s.IssueCode(ctx, "bob@example.com", PurposeVerify); !errors.Is(err, ErrCodeThrottled) {
		t.Fatal("second issue within a minute should throttle")
	}
	if _, err := s.IssueCode(ctx, "bob@example.com", PurposeReset); err != nil {
		t.Fatal("different purpose is independent:", err)
	}
	if err := s.CheckCode(ctx, "bob@example.com", PurposeVerify, "000000"); !errors.Is(err, ErrBadCode) && code != "000000" {
		t.Fatalf("bad code: %v", err)
	}
	if err := s.CheckCode(ctx, "bob@example.com", PurposeVerify, " "+code+" "); err != nil {
		t.Fatal("right code rejected:", err)
	}
	for i := 0; i < codeAttempts; i++ {
		s.CheckCode(ctx, "bob@example.com", PurposeVerify, "999999")
	}
	if err := s.CheckCode(ctx, "bob@example.com", PurposeVerify, code); !errors.Is(err, ErrCodeExpired) {
		t.Fatal("code should be dead after too many attempts")
	}
}

func TestPostsCommentsLikes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, _ := s.CreateUser(ctx, "author", "a@example.com", "password1", RoleAuthor, true)
	r, _ := s.CreateUser(ctx, "reader", "r@example.com", "password1", RoleReader, true)
	p := &Post{Slug: "hello-world", Title: "Hello World", BodyMD: "hi", BodyHTML: "<p>hi</p>", AuthorID: a.ID, Status: StatusPublished, PublishedAt: time.Now().Unix() - 10, CommentsEnabled: true, ReadingMin: 1}
	if err := s.SavePost(ctx, p, []string{"Go", "homelab", "go"}); err != nil {
		t.Fatal(err)
	}
	p2 := &Post{Slug: "hello-world", Title: "Second", AuthorID: a.ID, Status: StatusDraft}
	if err := s.SavePost(ctx, p2, nil); err != nil || p2.Slug != "hello-world-2" {
		t.Fatalf("slug dedupe: %q %v", p2.Slug, err)
	}
	sched := &Post{Slug: "future", Title: "Future", AuthorID: a.ID, Status: StatusPublished, PublishedAt: time.Now().Unix() + 3600}
	s.SavePost(ctx, sched, nil)

	live, total, err := s.ListPosts(ctx, PostFilter{Status: StatusPublished})
	if err != nil || total != 1 || live[0].Slug != "hello-world" {
		t.Fatalf("live posts = %d %v", total, err)
	}
	if len(live[0].Tags) != 2 {
		t.Fatalf("tags = %+v", live[0].Tags)
	}
	if _, n, _ := s.ListPosts(ctx, PostFilter{Status: "scheduled"}); n != 1 {
		t.Fatal("scheduled filter")
	}
	if _, n, _ := s.ListPosts(ctx, PostFilter{Status: StatusPublished, Query: "hello"}); n != 1 {
		t.Fatal("fts search miss")
	}
	if _, n, _ := s.ListPosts(ctx, PostFilter{Status: StatusPublished, Query: `"weird)(`}); n != 0 {
		t.Fatal("fts punctuation should not error")
	}
	if _, n, _ := s.ListPosts(ctx, PostFilter{Status: StatusPublished, Tag: "go"}); n != 1 {
		t.Fatal("tag filter")
	}
	tags, _ := s.Tags(ctx)
	if len(tags) != 2 {
		t.Fatalf("tag cloud = %+v", tags)
	}

	liked, n, err := s.ToggleLike(ctx, p.ID, r.ID)
	if err != nil || !liked || n != 1 {
		t.Fatalf("like: %v %v %d", err, liked, n)
	}
	liked, n, _ = s.ToggleLike(ctx, p.ID, r.ID)
	if liked || n != 0 {
		t.Fatal("unlike")
	}

	c1, err := s.AddComment(ctx, p.ID, r.ID, 0, "top", CommentVisible)
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := s.AddComment(ctx, p.ID, a.ID, c1, "reply", CommentVisible)
	c3, _ := s.AddComment(ctx, p.ID, r.ID, c2, "reply to reply", CommentPending)
	tree, n, _ := s.CommentsForPost(ctx, p.ID, 0, false)
	if n != 2 || len(tree) != 1 || len(tree[0].Replies) != 1 {
		t.Fatalf("anon tree: n=%d roots=%d", n, len(tree))
	}
	tree, n, _ = s.CommentsForPost(ctx, p.ID, r.ID, false)
	if n != 3 || len(tree[0].Replies) != 2 {
		t.Fatalf("own pending should show: n=%d", n)
	}
	if c, _ := s.CommentByID(ctx, c3); c.ParentID != c1 {
		t.Fatal("reply-to-reply should flatten to the root")
	}
	if got, _ := s.PostBySlug(ctx, "hello-world"); got.CommentCount != 2 {
		t.Fatalf("visible comment count = %d", got.CommentCount)
	}
	s.DeleteComment(ctx, c1)
	if _, n, _ = s.CommentsForPost(ctx, p.ID, 0, true); n != 0 {
		t.Fatal("deleting the root should remove replies")
	}
	if err := s.DeletePost(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if tags, _ := s.Tags(ctx); len(tags) != 0 {
		t.Fatal("orphan tags should be gone")
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Hello, World!":        "hello-world",
		"  Go 1.26 / Modules ": "go-1-26-modules",
		"Ünïcödé title":        "ncd-title",
		"---":                  "",
		"a__b   c":             "a-b-c",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
