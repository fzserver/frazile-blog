package web

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fzserver/frazile-blog/internal/store"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var (
	errTooBig  = errors.New("file is too large")
	errBadType = errors.New("unsupported file type")
)

var imageExt = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}
var videoExt = map[string]string{"video/mp4": ".mp4", "video/webm": ".webm", "video/quicktime": ".mov", "video/ogg": ".ogv"}

// sniff identifies the content type from the first bytes, never the
// client-supplied header. MP4 variants (isom, avc1, iso5, mp42...) all carry
// "ftyp" at offset 4, which Go's sniffer covers for the common brands; the
// rest are matched here so a phone recording is not refused.
func sniff(head []byte) string {
	ct := http.DetectContentType(head)
	ct = strings.Split(ct, ";")[0]
	if ct == "application/octet-stream" && len(head) > 12 && string(head[4:8]) == "ftyp" {
		brand := string(head[8:12])
		if brand == "qt  " {
			return "video/quicktime"
		}
		return "video/mp4"
	}
	if ct == "video/quicktime" || ct == "video/mp4" || ct == "video/webm" {
		return ct
	}
	return ct
}

// saveUpload stores one multipart file under MEDIA_DIR/YYYY/MM/<random>.<ext>
// and records it. Streams to disk, so a 1 GB video never sits in memory.
func (s *Server) saveUpload(r *http.Request, fh *multipart.FileHeader, allowVideo bool, uploader int64) (*store.Media, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	ct := sniff(head)
	ext, isImg := imageExt[ct]
	vext, isVid := videoExt[ct]
	var kind string
	var limit int64
	switch {
	case isImg:
		kind, limit = "image", int64(s.cfg.MaxImageMB)<<20
	case isVid && allowVideo:
		kind, ext, limit = "video", vext, int64(s.cfg.MaxVideoMB)<<20
	default:
		return nil, errBadType
	}
	if fh.Size > limit {
		return nil, errTooBig
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	rb := make([]byte, 8)
	rand.Read(rb)
	now := time.Now()
	rel := filepath.Join(now.Format("2006"), now.Format("01"), hex.EncodeToString(rb)+ext)
	abs := filepath.Join(s.cfg.MediaDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	out, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	size, err := io.Copy(out, io.LimitReader(f, limit+1))
	out.Close()
	if err != nil || size > limit {
		os.Remove(abs)
		if size > limit {
			return nil, errTooBig
		}
		return nil, err
	}
	m := &store.Media{Name: filepath.ToSlash(rel), OriginalName: filepath.Base(fh.Filename), Mime: ct, Kind: kind, Size: size, UploaderID: uploader}
	if kind == "image" {
		if img, err := os.Open(abs); err == nil {
			if cfg, _, err := image.DecodeConfig(img); err == nil {
				m.Width, m.Height = cfg.Width, cfg.Height
			}
			img.Close()
		}
	}
	if err := s.db.AddMedia(r.Context(), m); err != nil {
		os.Remove(abs)
		return nil, err
	}
	return m, nil
}

// saveAvatar decodes, centre-crops and scales an image to 256x256 JPEG at
// MEDIA_DIR/avatars/<user>.jpg. Returns the URL path with a cache-buster.
func (s *Server) saveAvatar(fh *multipart.FileHeader, userID int64) (string, error) {
	if fh.Size > 10<<20 {
		return "", errTooBig
	}
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 10<<20))
	if err != nil {
		return "", err
	}
	if _, ok := imageExt[sniff(data)]; !ok {
		return "", errBadType
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errBadType
	}
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, image.Rect(x0, y0, x0+side, y0+side), draw.Over, nil)
	dir := filepath.Join(s.cfg.MediaDir, "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d.jpg", userID)
	tmp := filepath.Join(dir, name+".tmp")
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: 88}); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", err
	}
	out.Close()
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/media/avatars/%s?v=%d", name, time.Now().Unix()), nil
}

// removeMediaFile deletes the on-disk file for a media row.
func (s *Server) removeMediaFile(name string) {
	clean := filepath.Clean("/" + name)
	os.Remove(filepath.Join(s.cfg.MediaDir, clean))
}
