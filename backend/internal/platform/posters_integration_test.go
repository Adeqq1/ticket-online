package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/posters"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
)

func TestPosterUploadSaveFailureAndPublicCatalog(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	auth := staffauth.New(db)
	suffix := randomHex(t, 8)
	id := "poster-roundtrip-" + suffix
	user, err := auth.CreateStaff(ctx, "Poster Roundtrip", "poster-roundtrip-"+suffix+"@example.test", "poster roundtrip password", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, query := range []string{"DELETE FROM admin_audit_log WHERE actor_id=?", "DELETE FROM staff_sessions WHERE staff_id=?", "DELETE FROM staff_users WHERE id=?"} {
			db.Exec(query, user.ID)
		}
		for _, query := range []string{"DELETE FROM event_lineups WHERE event_id=?", "DELETE FROM ticket_tiers WHERE event_id=?", "DELETE FROM event_zones WHERE event_id=?", "DELETE FROM events WHERE id=?"} {
			db.Exec(query, id)
		}
	}()
	if _, err = db.ExecContext(ctx, "UPDATE staff_users SET role='ADMIN' WHERE id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	session, err := auth.Login(ctx, user.Email, "poster roundtrip password", "192.0.2.8:4000")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	objects := map[string][]byte{}
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			objects[r.URL.Path] = data
			w.WriteHeader(200)
			return
		}
		data, ok := objects[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
			return
		}
		w.Write(data)
	}))
	defer s3Server.Close()
	t.Setenv("AWS_ACCESS_KEY_ID", "fixture")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fixture")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	posterHandler, err := posters.New(ctx, posters.Config{Bucket: "private-posters", Region: "us-east-1", Endpoint: s3Server.URL, Prefix: "posters", PublicOrigin: "https://tickets.example.test"}, auth, logger)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{AppEnv: "development", ReservationTTL: 10 * time.Minute, OrderAccessSecret: make([]byte, 32), MidtransEnvironment: "sandbox", TransactionsEnabled: true}
	handler := NewHandlerWithPosters(db, logger, cfg, nil, posterHandler)
	call := func(method, path string, data []byte, contentType string, admin bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		if admin {
			r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		}
		r.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	send := func(method, path string, value any, want int) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return call(method, path, data, "application/json", true, want)
	}
	input := map[string]any{"id": id, "artist": "Poster Roundtrip", "city": "Jakarta", "venue": "Test Venue", "address": "Test Address", "startsAt": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339), "genre": "Pop", "status": "Presale", "publicationStatus": "DRAFT", "image": "https://external.example.test/old.jpg", "description": "Description preserved", "lineup": []string{"Poster Roundtrip"}}
	send("POST", "/api/v1/admin/events", input, 201)
	send("POST", "/api/v1/admin/events/"+id+"/zones", map[string]any{"id": "main", "name": "Main", "description": "Main zone"}, 201)
	send("POST", "/api/v1/admin/events/"+id+"/ticket-tiers", map[string]any{"id": "main", "name": "Main", "zoneId": "main", "price": 10000, "capacity": 2, "maxPerOrder": 1, "benefit": "Main", "gate": "Main", "seating": "free-standing"}, 201)
	input["publicationStatus"] = "PUBLISHED"
	send("PUT", "/api/v1/admin/events/"+id, input, 200)
	var imageData, body bytes.Buffer
	if err = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 8, 10))); err != nil {
		t.Fatal(err)
	}
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../../poster.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imageData.Bytes())
	writer.Close()
	response := call("POST", "/api/v1/admin/posters", body.Bytes(), writer.FormDataContentType(), true, 201)
	var uploaded struct {
		URL string `json:"url"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	input["image"] = uploaded.URL
	input["genre"] = "invalid"
	send("PUT", "/api/v1/admin/events/"+id, input, 422)
	old := call("GET", "/api/v1/events/"+id, nil, "", false, 200)
	if !strings.Contains(old.Body.String(), "https://external.example.test/old.jpg") {
		t.Fatal("failed save replaced old poster")
	}
	input["genre"] = "Pop"
	send("PUT", "/api/v1/admin/events/"+id, input, 200)
	for _, path := range []string{"/api/v1/admin/events", "/api/v1/events", "/api/v1/events/" + id} {
		result := call("GET", path, nil, "", strings.Contains(path, "/admin/"), 200)
		if !strings.Contains(result.Body.String(), uploaded.URL) || !strings.Contains(result.Body.String(), "Description preserved") {
			t.Fatalf("poster/form not preserved on %s", path)
		}
	}
	media := call("GET", strings.TrimPrefix(uploaded.URL, "https://tickets.example.test"), nil, "", false, 200)
	if !bytes.Equal(media.Body.Bytes(), imageData.Bytes()) || media.Header().Get("Content-Type") != "image/png" {
		t.Fatal("public poster bytes or MIME incorrect")
	}
}
