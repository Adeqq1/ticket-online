package posters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/go-sql-driver/mysql"
)

func picture(t *testing.T, format string) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 10))
	var err error
	if format == "png" {
		err = png.Encode(&b, img)
	} else {
		err = jpeg.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func uploadRequest(t *testing.T, data []byte, filename string, count int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for range count {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/admin/posters", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
	return r
}

func fixture(t *testing.T, serve http.HandlerFunc) *Handler {
	t.Helper()
	server := httptest.NewServer(serve)
	t.Cleanup(server.Close)
	client := s3.New(s3.Options{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1})
	return &Handler{cfg: Config{Bucket: "private-posters", Region: "us-east-1", Prefix: "posters", PublicOrigin: "https://tickets.example.test"}, client: client, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), slots: make(chan struct{}, 2), authenticate: func(context.Context, string) (staffauth.Principal, error) {
		return staffauth.Principal{Staff: staffauth.Staff{ID: "admin", Role: "ADMIN"}}, nil
	}}
}

func resizedPNG(data []byte, width, height uint32) []byte {
	result := bytes.Clone(data)
	binary.BigEndian.PutUint32(result[16:20], width)
	binary.BigEndian.PutUint32(result[20:24], height)
	binary.BigEndian.PutUint32(result[29:33], crc32.ChecksumIEEE(result[12:29]))
	return result
}

func TestUploadValidationAndContentAddress(t *testing.T) {
	pngData := picture(t, "png")
	jpegData := picture(t, "jpeg")
	for _, tc := range []struct {
		name             string
		data             []byte
		filename         string
		count, want      int
		ext, contentType string
	}{
		{"PNG", pngData, "poster.png", 1, 201, ".png", "image/png"},
		{"JPEG despite misleading browser MIME and path filename", jpegData, "../../secret.png", 1, 201, ".jpg", "image/jpeg"},
		{"empty", nil, "a.png", 1, 422, "", ""},
		{"fake MIME", []byte("<svg onload='alert(1)'/>"), "a.png", 1, 422, "", ""},
		{"GIF", []byte("GIF89a"), "a.jpg", 1, 422, "", ""},
		{"corrupt", pngData[:len(pngData)-14], "a.png", 1, 422, "", ""},
		{"oversized", bytes.Repeat([]byte{0}, MaxFile+1), "a.png", 1, 413, "", ""},
		{"side too large", resizedPNG(pngData, 6001, 10), "a.png", 1, 422, "", ""},
		{"too many pixels", resizedPNG(pngData, 5000, 5000), "a.png", 1, 422, "", ""},
		{"multiple files", pngData, "a.png", 2, 400, "", ""},
		{"missing file", pngData, "a.png", 0, 400, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var writes int
			var stored []byte
			var path, contentType string
			h := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				writes++
				path = r.URL.Path
				contentType = r.Header.Get("Content-Type")
				stored, _ = io.ReadAll(r.Body)
				w.WriteHeader(200)
			})
			response := httptest.NewRecorder()
			h.Upload(response, uploadRequest(t, tc.data, tc.filename, tc.count))
			if response.Code != tc.want {
				t.Fatalf("status=%d want=%d: %s", response.Code, tc.want, response.Body.String())
			}
			if tc.want != 201 {
				if writes != 0 {
					t.Fatal("invalid file stored")
				}
				return
			}
			name := strings.TrimPrefix(path, "/private-posters/posters/")
			expected := sha256.Sum256(tc.data)
			if name != fmtHash(expected)+tc.ext || contentType != tc.contentType || !bytes.Equal(stored, tc.data) {
				t.Fatalf("unexpected storage: %s %s (%d bytes)", path, contentType, len(stored))
			}
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["url"] != "https://tickets.example.test/api/v1/media/posters/"+name {
				t.Fatal(result)
			}
		})
	}
}

func fmtHash(hash [32]byte) string { return fmt.Sprintf("%x", hash) }

type unreadableBody struct{ read bool }

func (b *unreadableBody) Read([]byte) (int, error) {
	b.read = true
	return 0, errors.New("body must not be read")
}
func (*unreadableBody) Close() error { return nil }

func TestAuthenticationBeforeReadingAndUnavailableStorage(t *testing.T) {
	for _, tc := range []struct {
		name, role        string
		authErr           error
		token, configured bool
		want              int
	}{
		{"anonymous", "", nil, false, true, 401}, {"expired", "", staffauth.ErrUnauthorized, true, true, 401}, {"staff", "STAFF", nil, true, true, 403}, {"auth outage", "", errors.New("DB failed"), true, true, 503}, {"unconfigured", "ADMIN", nil, true, false, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("S3 reached") })
			if !tc.configured {
				h.client = nil
			}
			h.authenticate = func(context.Context, string) (staffauth.Principal, error) {
				return staffauth.Principal{Staff: staffauth.Staff{Role: tc.role}}, tc.authErr
			}
			body := &unreadableBody{}
			r := httptest.NewRequest("POST", "/api/v1/admin/posters", nil)
			r.Body = body
			if tc.token {
				r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
			}
			response := httptest.NewRecorder()
			h.Upload(response, r)
			if response.Code != tc.want || body.read || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status %d; read=%v", response.Code, body.read)
			}
		})
	}
}

func TestWholeRequestLimitAndWrongField(t *testing.T) {
	h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("S3 reached") })
	for _, kind := range []string{"length", "chunked epilogue", "wrong field", "non multipart"} {
		t.Run(kind, func(t *testing.T) {
			r := uploadRequest(t, picture(t, "png"), "poster.png", 1)
			switch kind {
			case "length":
				r.ContentLength = MaxRequest + 1
			case "chunked epilogue":
				data, _ := io.ReadAll(r.Body)
				data = append(data, bytes.Repeat([]byte("x"), MaxRequest)...)
				r.Body = io.NopCloser(bytes.NewReader(data))
				r.ContentLength = -1
			case "wrong field":
				data, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(bytes.Replace(data, []byte(`name="file"`), []byte(`name="other"`), 1)))
			case "non multipart":
				r.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			h.Upload(response, r)
			want := 400
			if kind == "length" || kind == "chunked epilogue" {
				want = 413
			}
			if response.Code != want {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestStorageFailureTimeoutAndConcurrency(t *testing.T) {
	var writes atomic.Int32
	entered, release := make(chan struct{}, 2), make(chan struct{})
	h := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		writes.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(500)
	})
	defer close(release)
	done := make(chan int, 2)
	for range 2 {
		r := uploadRequest(t, picture(t, "png"), "a.png", 1)
		go func() { w := httptest.NewRecorder(); h.Upload(w, r); done <- w.Code }()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("upload did not start")
		}
	}
	busy := httptest.NewRecorder()
	h.Upload(busy, uploadRequest(t, picture(t, "png"), "a.png", 1))
	if busy.Code != 429 || busy.Header().Get("Retry-After") == "" {
		t.Fatal(busy.Code)
	}
	// Cancel an actual SDK operation while it is waiting for S3.
	timeout := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	r := uploadRequest(t, picture(t, "png"), "a.png", 1)
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	timeout.Upload(w, r.WithContext(ctx))
	if w.Code != 504 || strings.Contains(w.Body.String(), `"url"`) {
		t.Fatalf("timeout: %d %s", w.Code, w.Body.String())
	}
	// Release both in-flight uploads and verify S3 failures return no URL.
	release <- struct{}{}
	release <- struct{}{}
	for range 2 {
		if status := <-done; status != 503 {
			t.Fatalf("S3 failure status %d", status)
		}
	}
	if writes.Load() != 2 {
		t.Fatal(writes.Load())
	}
}

func TestMediaHeadersCachingMissingAndInvalidPath(t *testing.T) {
	data := picture(t, "png")
	name := fmtHash(sha256.Sum256(data)) + ".png"
	h := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, strings.Repeat("0", 64)+".png") {
			w.WriteHeader(404)
			io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	})
	mux := http.NewServeMux()
	h.Register(mux)
	for _, tc := range []struct {
		name string
		want int
	}{{name, 200}, {strings.Repeat("0", 64) + ".png", 404}, {"bad.svg", 404}, {"%2E%2E%2Fsecret.png", 404}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/media/posters/"+tc.name, nil))
		if w.Code != tc.want {
			t.Fatalf("%s: %d %s", tc.name, w.Code, w.Body.String())
		}
		if tc.want == 200 && (w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") || !bytes.Equal(w.Body.Bytes(), data)) {
			t.Fatal(w.Header())
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/media/posters/"+name, nil)
	r.Header.Set("If-None-Match", `"`+name+`"`)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 304 {
		t.Fatal(w.Code)
	}
}

func TestRealSessions(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	auth := staffauth.New(db)
	user, err := auth.CreateStaff(ctx, "Poster Admin", "poster-"+time.Now().Format("150405.000000000")+"@example.test", "poster integration password", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DELETE FROM staff_users WHERE id = ?", user.ID)
	defer db.Exec("DELETE FROM staff_sessions WHERE staff_id = ?", user.ID)
	session, err := auth.Login(ctx, user.Email, "poster integration password", "192.0.2.9:4000")
	if err != nil {
		t.Fatal(err)
	}
	h := fixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h.authenticate = auth.Authenticate
	call := func(want int) {
		t.Helper()
		r := uploadRequest(t, picture(t, "png"), "a.png", 1)
		r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		w := httptest.NewRecorder()
		h.Upload(w, r)
		if w.Code != want {
			t.Fatalf("%d want %d: %s", w.Code, want, w.Body.String())
		}
	}
	call(403)
	if _, err = db.ExecContext(ctx, "UPDATE staff_users SET role='ADMIN' WHERE id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	call(201)
	if _, err = db.ExecContext(ctx, "UPDATE staff_sessions SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE staff_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	call(401)
}

func TestStagingS3(t *testing.T) {
	bucket := os.Getenv("POSTER_S3_TEST_BUCKET")
	if bucket == "" {
		t.Skip("set POSTER_S3_TEST_BUCKET and POSTER_S3_TEST_REGION for staging S3 integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg := Config{Bucket: bucket, Region: os.Getenv("POSTER_S3_TEST_REGION"), Endpoint: os.Getenv("POSTER_S3_TEST_ENDPOINT"), Prefix: os.Getenv("POSTER_S3_TEST_PREFIX"), PublicOrigin: "https://staging.example.test"}
	h, err := New(ctx, cfg, staffauth.New(nil), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	versioning, err := h.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatal(err)
	}
	if string(versioning.Status) != "Enabled" {
		t.Fatal("bucket versioning must be enabled")
	}
	h.authenticate = func(context.Context, string) (staffauth.Principal, error) {
		return staffauth.Principal{Staff: staffauth.Staff{Role: "ADMIN"}}, nil
	}
	data := picture(t, "png")
	w := httptest.NewRecorder()
	h.Upload(w, uploadRequest(t, data, "staging-check.png", 1))
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	// A new handler/client represents a redeploy: the content URL must still work.
	restarted, err := New(ctx, cfg, staffauth.New(nil), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	restarted.Register(mux)
	r := httptest.NewRequest("GET", "/api/v1/media/posters/"+fmtHash(sha256.Sum256(data))+".png", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("read after restart: %d", w.Code)
	}
}
