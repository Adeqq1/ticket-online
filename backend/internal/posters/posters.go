package posters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const MaxFile = 5 << 20
const MaxRequest = 6 << 20
const Timeout = 30 * time.Second

var filenamePattern = regexp.MustCompile(`^[0-9a-f]{64}\.(jpg|png)$`)

type Config struct {
	Bucket, Region, Endpoint, Prefix, PublicOrigin string
}

func (c Config) Validate() error {
	if c.Bucket == "" {
		return nil
	}
	u, err := url.Parse(c.PublicOrigin)
	if c.Region == "" || err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || len(c.PublicOrigin)+len("/api/v1/media/posters/")+68 > 500 {
		return fmt.Errorf("poster storage requires region and a valid public origin (maximum concert image URL: 500 bytes)")
	}
	if strings.ContainsAny(c.Prefix, "\\\r\n") {
		return fmt.Errorf("invalid poster object prefix")
	}
	for _, part := range strings.Split(c.Prefix, "/") {
		if part == "." || part == ".." {
			return fmt.Errorf("invalid poster object prefix")
		}
	}
	if c.Endpoint != "" {
		u, err := url.Parse(c.Endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid poster S3 endpoint")
		}
	}
	return nil
}

type Handler struct {
	cfg          Config
	client       *s3.Client
	authenticate func(context.Context, string) (staffauth.Principal, error)
	logger       *slog.Logger
	slots        chan struct{}
}

func New(ctx context.Context, cfg Config, auth *staffauth.Service, logger *slog.Logger) (*Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	h := &Handler{cfg: cfg, authenticate: auth.Authenticate, logger: logger, slots: make(chan struct{}, 2)}
	if cfg.Bucket == "" {
		return h, nil
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region), config.WithRetryMaxAttempts(2))
	if err != nil {
		return nil, fmt.Errorf("load poster AWS configuration: %w", err)
	}
	h.client = s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})
	return h, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/posters", h.Upload)
	mux.HandleFunc("GET /api/v1/media/posters/{filename}", h.Media)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if r.Method == http.MethodPost {
		h.logger.WarnContext(r.Context(), "poster upload failed", "code", code, "status", status)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (h *Handler) key(filename string) string {
	if prefix := strings.Trim(h.cfg.Prefix, "/"); prefix != "" {
		return prefix + "/" + filename
	}
	return filename
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), Timeout)
	defer cancel()
	r = r.WithContext(ctx)
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		h.fail(w, r, 401, "UNAUTHORIZED", "Sesi admin diperlukan")
		return
	}
	principal, err := h.authenticate(ctx, token)
	if err != nil {
		if errors.Is(err, staffauth.ErrUnauthorized) {
			h.fail(w, r, 401, "UNAUTHORIZED", "Sesi tidak valid atau sudah berakhir")
		} else {
			h.fail(w, r, 503, "AUTH_UNAVAILABLE", "Autentikasi belum tersedia. Coba lagi.")
		}
		return
	}
	if principal.Role != "ADMIN" {
		h.fail(w, r, 403, "FORBIDDEN", "Akses administrator diperlukan")
		return
	}
	if h.client == nil {
		h.fail(w, r, 503, "POSTERS_UNAVAILABLE", "Upload poster belum dikonfigurasi. Gunakan URL gambar.")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		w.Header().Set("Retry-After", "3")
		h.fail(w, r, 429, "UPLOAD_BUSY", "Upload sedang sibuk. Coba lagi.")
		return
	}
	if r.ContentLength > MaxRequest {
		h.fail(w, r, 413, "POSTER_TOO_LARGE", "File maksimal 5 MiB dan request maksimal 6 MiB")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequest)
	reader, err := r.MultipartReader()
	if err != nil {
		h.multipartError(w, r, err)
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		h.multipartError(w, r, err)
		return
	}
	if part.FormName() != "file" || part.FileName() == "" {
		h.multipartError(w, r, nil)
		return
	}
	data, err := io.ReadAll(io.LimitReader(part, MaxFile+1))
	if err != nil {
		h.multipartError(w, r, err)
		return
	}
	if len(data) > MaxFile {
		h.fail(w, r, 413, "POSTER_TOO_LARGE", "File poster maksimal 5 MiB")
		return
	}
	if _, err = reader.NextPart(); err != io.EOF {
		h.multipartError(w, r, err)
		return
	}
	// Drain the epilogue too: the 6 MiB ceiling applies to the entire request.
	if _, err = io.Copy(io.Discard, r.Body); err != nil {
		h.multipartError(w, r, err)
		return
	}
	filename, contentType, err := validate(data)
	if err != nil {
		h.fail(w, r, 422, "INVALID_POSTER", err.Error())
		return
	}
	_, err = h.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(h.cfg.Bucket), Key: aws.String(h.key(filename)), Body: bytes.NewReader(data), ContentType: aws.String(contentType), CacheControl: aws.String("public, max-age=31536000, immutable")})
	if err != nil {
		h.storageError(w, r, ctx)
		return
	}
	h.logger.InfoContext(ctx, "poster upload succeeded", "staff_id", principal.ID, "filename", filename, "bytes", len(data))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"url": h.cfg.PublicOrigin + "/api/v1/media/posters/" + filename})
}

func validate(data []byte) (string, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return "", "", fmt.Errorf("Pilih gambar JPEG atau PNG yang valid (HEIC, SVG, dan GIF tidak didukung)")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 6000 || cfg.Height > 6000 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return "", "", fmt.Errorf("Dimensi maksimal 6.000 piksel per sisi dan total 20 megapiksel")
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return "", "", fmt.Errorf("File gambar rusak atau tidak lengkap")
	}
	ext, contentType := ".png", "image/png"
	if format == "jpeg" {
		ext, contentType = ".jpg", "image/jpeg"
	}
	return fmt.Sprintf("%x%s", sha256.Sum256(data), ext), contentType, nil
}

func (h *Handler) multipartError(w http.ResponseWriter, r *http.Request, err error) {
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		h.fail(w, r, 413, "POSTER_TOO_LARGE", "Request maksimal 6 MiB")
		return
	}
	h.fail(w, r, 400, "INVALID_MULTIPART", "Kirim multipart/form-data dengan tepat satu file pada field file")
}

func (h *Handler) storageError(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		h.fail(w, r, 504, "POSTER_TIMEOUT", "Penyimpanan poster melewati batas waktu. Coba lagi.")
		return
	}
	h.fail(w, r, 503, "POSTER_STORAGE_FAILED", "Penyimpanan poster belum tersedia. Coba lagi.")
}

func (h *Handler) Media(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	filename := r.PathValue("filename")
	if !filenamePattern.MatchString(filename) {
		h.fail(w, r, 404, "POSTER_NOT_FOUND", "Poster tidak ditemukan")
		return
	}
	if h.client == nil {
		h.fail(w, r, 503, "POSTERS_UNAVAILABLE", "Penyimpanan poster belum dikonfigurasi")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), Timeout)
	defer cancel()
	object, err := h.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(h.cfg.Bucket), Key: aws.String(h.key(filename))})
	if err != nil {
		var missing *types.NoSuchKey
		if errors.As(err, &missing) {
			h.fail(w, r, 404, "POSTER_NOT_FOUND", "Poster tidak ditemukan")
		} else {
			h.storageError(w, r, ctx)
		}
		return
	}
	defer object.Body.Close()
	data, err := io.ReadAll(io.LimitReader(object.Body, MaxFile+1))
	if err != nil || len(data) > MaxFile || len(data) == 0 {
		h.storageError(w, r, ctx)
		return
	}
	contentType := "image/png"
	if strings.HasSuffix(filename, ".jpg") {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+filename+`"`)
	http.ServeContent(w, r, filename, time.Time{}, bytes.NewReader(data))
}
