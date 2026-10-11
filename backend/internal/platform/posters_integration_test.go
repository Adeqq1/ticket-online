package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
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

func TestPhase40DummyConcertPosterPurchaseAndTwoStaff(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	users := []string{user.ID}
	reservationID, orderID := "", ""
	defer func() {
		remove := func(query, value string) {
			if _, err := db.Exec(query, value); err != nil {
				t.Errorf("fixture cleanup %s: %v", query, err)
			}
		}
		for _, table := range []string{"checkin_attempts", "ticket_checkins"} {
			remove("DELETE FROM "+table+" WHERE event_id=?", id)
		}
		for _, userID := range users {
			for _, query := range []string{"DELETE FROM admin_audit_log WHERE actor_id=?", "DELETE FROM staff_sessions WHERE staff_id=?", "DELETE FROM staff_assignments WHERE staff_id=?", "DELETE FROM staff_users WHERE id=?"} {
				remove(query, userID)
			}
		}
		for _, table := range []string{"email_queue", "etickets", "order_attendees", "payments", "order_items", "order_buyers"} {
			remove("DELETE FROM "+table+" WHERE order_id=?", orderID)
		}
		remove("DELETE FROM orders WHERE id=?", orderID)
		remove("DELETE FROM reservation_items WHERE reservation_id=?", reservationID)
		remove("DELETE FROM reservations WHERE id=?", reservationID)
		for _, query := range []string{"DELETE FROM event_lineups WHERE event_id=?", "DELETE FROM ticket_tiers WHERE event_id=?", "DELETE FROM event_zones WHERE event_id=?", "DELETE FROM events WHERE id=?"} {
			remove(query, id)
		}
	}()
	if _, err = db.ExecContext(ctx, "UPDATE staff_users SET role='ADMIN' WHERE id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	// ponytail: external S3 is an HTTP fixture; use TestStagingS3 for provider durability/IAM.
	objects := map[string][]byte{}
	storageOffline := false
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			if storageOffline {
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				connection.Close()
				return
			}
			if _, exists := objects[r.URL.Path]; exists && r.Header.Get("If-None-Match") == "*" {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
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
	posterConfig := posters.Config{Bucket: "private-posters", Region: "us-east-1", Endpoint: s3Server.URL, Prefix: "posters", PublicOrigin: "https://tickets.example.test"}
	posterHandler, err := posters.New(ctx, posterConfig, auth, logger)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{AppEnv: "development", ReservationTTL: 10 * time.Minute, OrderAccessSecret: make([]byte, 32), MidtransEnvironment: "sandbox", TransactionsEnabled: true}
	handler := NewHandlerWithPosters(db, logger, cfg, nil, posterHandler)
	apiServer := httptest.NewServer(handler)
	defer func() { apiServer.Close() }()
	token := ""
	call := func(method, path string, data []byte, contentType string, admin bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, apiServer.URL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if admin {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Content-Type", contentType)
		if method == "POST" && strings.HasPrefix(path, "/api/v1/reservations") {
			r.Header.Set("Idempotency-Key", id)
		}
		w := httptest.NewRecorder()
		response, err := apiServer.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		w.HeaderMap = response.Header.Clone()
		w.WriteHeader(response.StatusCode)
		if _, err := io.Copy(w, response.Body); err != nil {
			t.Fatal(err)
		}
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
	login := func(email string) string {
		t.Helper()
		var result staffauth.Session
		response := send("POST", "/api/v1/staff/login", map[string]string{"email": email, "password": "poster roundtrip password"}, 200)
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.AccessToken == "" {
			t.Fatalf("login failed: %v", err)
		}
		return result.AccessToken
	}
	token = login(user.Email)
	input := map[string]any{"id": id, "artist": "Poster Roundtrip", "city": "Jakarta", "venue": "Test Venue", "address": "Test Address", "startsAt": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339), "genre": "Pop", "status": "Presale", "publicationStatus": "DRAFT", "image": "https://external.example.test/old.jpg", "description": "Description preserved", "lineup": []string{"Poster Roundtrip"}}
	send("POST", "/api/v1/admin/events", input, 201)
	send("POST", "/api/v1/admin/events/"+id+"/zones", map[string]any{"id": "main", "name": "Main", "description": "Main zone"}, 201)
	send("POST", "/api/v1/admin/events/"+id+"/ticket-tiers", map[string]any{"id": "main", "name": "Main", "zoneId": "main", "price": 10000, "capacity": 2, "maxPerOrder": 2, "benefit": "Main", "gate": "Main", "seating": "free-standing"}, 201)
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
	t.Log("ADMIN 1: draft, zone, tier, publish, upload, failed save preserved old poster, successful save visible publicly")

	// Only fixture setup promotes the second admin; all operational tasks use HTTP.
	secondAdmin, err := auth.CreateStaff(ctx, "Dummy Admin 2", "phase40-admin2-"+suffix+"@example.test", "poster roundtrip password", nil)
	if err != nil {
		t.Fatal(err)
	}
	users = append(users, secondAdmin.ID)
	if _, err := db.ExecContext(ctx, "UPDATE staff_users SET role='ADMIN' WHERE id=?", secondAdmin.ID); err != nil {
		t.Fatal(err)
	}
	token = login(secondAdmin.Email)
	oldURL := uploaded.URL
	replacement := image.NewRGBA(image.Rect(0, 0, 8, 10))
	replacement.Set(0, 0, color.RGBA{R: 255, A: 255})
	imageData.Reset()
	if err := png.Encode(&imageData, replacement); err != nil {
		t.Fatal(err)
	}
	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "../../replacement.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imageData.Bytes())
	writer.Close()
	mu.Lock()
	storageOffline = true
	mu.Unlock()
	failed := call("POST", "/api/v1/admin/posters", body.Bytes(), writer.FormDataContentType(), true, 503)
	if strings.Contains(failed.Body.String(), `"url"`) {
		t.Fatal("failed upload returned a URL")
	}
	if result := call("GET", "/api/v1/events/"+id, nil, "", false, 200); !strings.Contains(result.Body.String(), oldURL) {
		t.Fatal("disconnected upload replaced saved poster")
	}
	mu.Lock()
	storageOffline = false
	mu.Unlock()
	response = call("POST", "/api/v1/admin/posters", body.Bytes(), writer.FormDataContentType(), true, 201)
	if err := json.Unmarshal(response.Body.Bytes(), &uploaded); err != nil || uploaded.URL == oldURL {
		t.Fatalf("replacement URL invalid: %v", err)
	}
	input["image"] = uploaded.URL
	send("PUT", "/api/v1/admin/events/"+id, input, 200)
	call("GET", strings.TrimPrefix(oldURL, posterConfig.PublicOrigin), nil, "", false, 200)
	// Restart all API handlers and SDK clients while the external store and DB survive.
	apiServer.Close()
	posterHandler, err = posters.New(ctx, posterConfig, staffauth.New(db), logger)
	if err != nil {
		t.Fatal(err)
	}
	apiServer = httptest.NewServer(NewHandlerWithPosters(db, logger, cfg, nil, posterHandler))
	result := call("GET", "/api/v1/events/"+id, nil, "", false, 200)
	if !strings.Contains(result.Body.String(), uploaded.URL) {
		t.Fatal("restart lost saved poster URL")
	}
	media = call("GET", strings.TrimPrefix(uploaded.URL, posterConfig.PublicOrigin), nil, "", false, 200)
	if !bytes.Equal(media.Body.Bytes(), imageData.Bytes()) || media.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(media.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("restart lost poster bytes or safe cache headers")
	}
	call("GET", strings.TrimPrefix(oldURL, posterConfig.PublicOrigin), nil, "", false, 200)
	result = call("GET", "/api/v1/events", nil, "", false, 200)
	if !strings.Contains(result.Body.String(), uploaded.URL) {
		t.Fatal("catalog lost replacement poster after restart")
	}
	t.Log("ADMIN 2: disconnected storage retained poster, retry and replacement succeeded, old/new assets survive API restart")

	staffTokens := []string{}
	for i := 1; i <= 2; i++ {
		email := fmt.Sprintf("phase40-staff%d-%s@example.test", i, suffix)
		response := send("POST", "/api/v1/admin/staff", map[string]any{"name": fmt.Sprintf("Dummy Staff %d", i), "email": email, "password": "poster roundtrip password", "assignments": []staffauth.Assignment{{EventID: id, Gate: "Main"}}}, 201)
		var created staffauth.Staff
		if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.ID == "" {
			t.Fatalf("create staff failed: %v", err)
		}
		users = append(users, created.ID)
		staffTokens = append(staffTokens, login(email))
	}
	reserved := send("POST", "/api/v1/reservations", map[string]any{"eventId": id, "items": []map[string]any{{"tierId": "main", "quantity": 2}}}, 201)
	var reservation struct{ ID string }
	if err := json.Unmarshal(reserved.Body.Bytes(), &reservation); err != nil || reservation.ID == "" {
		t.Fatalf("reservation failed: %v", err)
	}
	reservationID = reservation.ID
	ordered := send("POST", "/api/v1/reservations/"+reservationID+"/checkout", map[string]any{"buyer": map[string]string{"name": "Dummy Buyer", "email": "phase40-buyer@example.test", "phone": "081234567890", "identity": "123456789012"}, "attendees": []map[string]any{{"tierId": "main", "names": []string{"Dummy Guest 1", "Dummy Guest 2"}}}}, 201)
	var order struct{ ID, AccessToken string }
	if err := json.Unmarshal(ordered.Body.Bytes(), &order); err != nil || order.ID == "" || order.AccessToken == "" {
		t.Fatalf("checkout failed: %v", err)
	}
	orderID = order.ID
	token = order.AccessToken
	paid := send("POST", "/api/v1/orders/"+orderID+"/simulate-payment", map[string]string{"method": "QRIS", "result": "SUCCEEDED"}, 201)
	var payment struct{ Tickets []struct{ Code string } }
	if err := json.Unmarshal(paid.Body.Bytes(), &payment); err != nil || len(payment.Tickets) != 2 {
		t.Fatalf("ticket issuance failed: %v", err)
	}
	code := payment.Tickets[0].Code
	token = staffTokens[0]
	call("POST", "/api/v1/admin/posters", body.Bytes(), writer.FormDataContentType(), true, 403)
	checkin := map[string]string{"eventId": id, "gate": "Main", "code": code}
	send("POST", "/api/v1/staff/check-ins", checkin, 201)
	token = staffTokens[1]
	duplicate := send("POST", "/api/v1/staff/check-ins", checkin, 409)
	if !strings.Contains(duplicate.Body.String(), "TICKET_ALREADY_USED") {
		t.Fatal("second staff did not see duplicate check-in")
	}
	status := call("GET", "/api/v1/staff/ticket-status?code="+code, nil, "", true, 200)
	if !strings.Contains(status.Body.String(), "CHECKED_IN") {
		t.Fatal("second staff could not recover check-in status")
	}
	checkin["code"] = payment.Tickets[1].Code
	send("POST", "/api/v1/staff/check-ins", checkin, 201)
	if _, err := db.ExecContext(ctx, "UPDATE staff_sessions SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE staff_id=?", users[len(users)-1]); err != nil {
		t.Fatal(err)
	}
	send("POST", "/api/v1/staff/check-ins", checkin, 401)
	call("POST", "/api/v1/admin/posters", body.Bytes(), writer.FormDataContentType(), false, 401)
	t.Log("dummy buyer: checkout/payment/ticket; STAFF 1: check-in; STAFF 2: duplicate/status recovery/expired session; upload role denials")
}
