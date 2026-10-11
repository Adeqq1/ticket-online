package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/adminissues"
	"github.com/Adeqq1/ticket-online/backend/internal/email"
	"github.com/Adeqq1/ticket-online/backend/internal/operations"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

// Uses the paid test order and local SMTP sink from the delivery integration.
// PHASE58_BROWSER_URL adds a real browser, relaying HTTP to this isolated test server.
func verifyAdminEmailHandling(t *testing.T, db *sql.DB, ctx context.Context, secret []byte, orderID, reference, adminToken string, port int, mails <-chan capturedMail) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	execSQL := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	var ticketJob, buyer string
	if err := db.QueryRowContext(ctx, "SELECT id,recipient FROM email_queue WHERE kind='TICKETS' AND order_id=?", orderID).Scan(&ticketJob, &buyer); err != nil {
		t.Fatal(err)
	}
	execSQL("UPDATE email_queue SET status='FAILED',attempts=1,last_error='SMTP koneksi gagal',updated_at=UTC_TIMESTAMP(6) WHERE id=?", ticketJob)
	access := orderaccess.New(db, secret)
	pair := recovery.PairKey(access, buyer, reference)
	// A prior buyer resend/recovery in the parent scenario used these same cooldown buckets.
	execSQL("DELETE FROM rate_limits WHERE bucket IN (?,?,?)", access.Digest("limit/resend-order", orderID), access.Digest("limit/recovery-pair-cooldown", pair), access.Digest("limit/recovery-pair", pair))
	request, blocked := randomHex(t, 16), randomHex(t, 16)
	for _, id := range []string{request, blocked} {
		execSQL("INSERT INTO recovery_requests(id,reference,pair_key,expires_at,created_at) VALUES (?,?,?,UTC_TIMESTAMP(6)+INTERVAL 10 MINUTE,UTC_TIMESTAMP(6))", id, reference, pair)
		execSQL("INSERT INTO email_queue(id,kind,dedupe_key,recovery_request_id,recipient,status,attempts,next_attempt_at,last_error,created_at,updated_at) VALUES (?,'RECOVERY',?,?,'','FAILED',3,UTC_TIMESTAMP(6),'permintaan pemulihan kedaluwarsa',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))", id, "recovery:"+id, id)
	}
	execSQL("UPDATE recovery_requests SET used_at=UTC_TIMESTAMP(6) WHERE id=?", blocked)
	// A replaced failure and every non-failure status must stay out of the failure list.
	for _, status := range []string{"PENDING", "PROCESSING", "SENT", "FAILED"} {
		id := randomHex(t, 16)
		execSQL("INSERT INTO email_queue(id,kind,dedupe_key,recovery_request_id,recipient,status,attempts,next_attempt_at,last_error,superseded_by,created_at,updated_at) VALUES (?,'RECOVERY',?,?,'',?,3,UTC_TIMESTAMP(6)+INTERVAL 1 DAY,'SMTP koneksi gagal',?,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))", id, "fixture:"+id, blocked, status, func() any {
			if status == "FAILED" {
				return request
			}
			return nil
		}())
	}
	staff := staffauth.New(db)
	metrics := operations.NewService(db, staff, operations.NewTracker())
	before, err := metrics.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.FailedEmailJobs != 1 {
		t.Fatalf("monitoring before retry = %d, want 1", before.FailedEmailJobs)
	}
	handler := NewHandlerWithOperations(db, logger, 10*time.Minute, "", false, secret, "sandbox-test-key", "http://localhost:5173", metrics)
	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/admin/"+path, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	list := func() []adminissues.EmailJob {
		res := call("GET", "email-jobs")
		if res.Code != 200 {
			t.Fatalf("list: %d %s", res.Code, res.Body.String())
		}
		var page struct{ Items []adminissues.EmailJob }
		if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	rows := list()
	if len(rows) != 3 {
		t.Fatalf("failure list contains %d entries, want 3", len(rows))
	}
	for _, row := range rows {
		if row.Status != "FAILED" || row.SupersededBy != nil || row.Recipient == buyer {
			t.Fatalf("wrong list scope or unmasked recipient: %s", row.ID)
		}
	}
	if res := call("POST", "email-jobs/"+blocked+"/retry"); res.Code != 409 {
		t.Fatalf("retry prohibited job: %d", res.Code)
	}
	for _, id := range []string{ticketJob, request, blocked} {
		res := call("GET", "email-jobs/"+id)
		var detail adminissues.EmailDetail
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &detail) != nil || detail.CanRetry == (id == blocked) {
			t.Fatalf("retry availability: %d %s", res.Code, res.Body.String())
		}
	}
	// Remove non-failed fixtures before running the worker; they only test list eligibility.
	execSQL("DELETE FROM email_queue WHERE dedupe_key LIKE 'fixture:%' AND recovery_request_id=?", blocked)
	for len(mails) > 0 {
		<-mails
	}
	browserURL := os.Getenv("PHASE58_BROWSER_URL")
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	workerDone := make(chan struct{})
	startWorker := func() {
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-ticker.C:
					if _, err := metrics.Collect(workerCtx); err != nil && workerCtx.Err() == nil {
						logger.Error("test snapshot", "error", err)
					}
				}
			}
		}()
		go func() {
			defer close(workerDone)
			email.NewService(db, email.Config{Host: "127.0.0.1", Port: port, From: "tickets@example.test", TLSMode: "none", FrontendURL: "http://localhost:5173", AccessSecret: secret}, logger).Run(workerCtx, 50*time.Millisecond)
		}()
	}
	if browserURL != "" {
		// Commit through the actual handler, then sever only the successful retry response.
		// The browser must reconcile from real status/history without another POST.
		interrupted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v1/admin/email-jobs/") && strings.HasSuffix(r.URL.Path, "/retry") {
				result := httptest.NewRecorder()
				handler.ServeHTTP(result, r)
				if result.Code == 200 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
					return
				}
				for key, values := range result.Header() {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(result.Code)
				_, _ = w.Write(result.Body.Bytes())
				return
			}
			handler.ServeHTTP(w, r)
		})
		server := httptest.NewServer(interrupted)
		defer server.Close()
		signal := filepath.Join(t.TempDir(), "start-worker")
		jobs, _ := json.Marshal([]string{ticketJob, request})
		cmd := exec.CommandContext(ctx, "python3", "scripts/email-ui-check.py", "--base-url", browserURL, "--real-api", server.URL, "--output-dir", "/tmp/phase56-58/backend")
		cmd.Dir = "../../../frontend"
		cmd.Env = append(os.Environ(), "PHASE58_TOKEN="+adminToken, "PHASE58_JOBS="+string(jobs), "PHASE58_WORKER_SIGNAL="+signal, "PHASE58_DROP_RESPONSE=true")
		// Delay the worker until the browser has recorded both scheduled outcomes.
		started := make(chan bool, 1)
		go func() {
			for {
				if _, err := os.Stat(signal); err == nil {
					startWorker()
					started <- true
					return
				}
				select {
				case <-workerCtx.Done():
					started <- false
					return
				case <-time.After(25 * time.Millisecond):
				}
			}
		}()
		output, err := cmd.CombinedOutput()
		if err != nil {
			stop()
			if <-started {
				<-workerDone
			}
			t.Fatalf("real browser: %v\n%s", err, output)
		}
		if !<-started {
			t.Fatal("browser never scheduled worker")
		}
		t.Log(strings.TrimSpace(string(output)))
	} else {
		for _, id := range []string{ticketJob, request} {
			res := call("POST", "email-jobs/"+id+"/retry")
			if res.Code != 200 {
				t.Fatalf("schedule: %d %s", res.Code, res.Body.String())
			}
		}
		if res := call("POST", "email-jobs/"+ticketJob+"/retry"); res.Code != 409 {
			t.Fatalf("duplicate schedule: %d", res.Code)
		}
		startWorker()
	}
	// The same job ID is reused for tickets; recovery produces a new job and RETRY evidence.
	var replacement string
	if err := db.QueryRowContext(ctx, "SELECT superseded_by FROM email_queue WHERE id=?", request).Scan(&replacement); err != nil || replacement == request || replacement == "" {
		t.Fatalf("replacement missing: %v", err)
	}
	for _, id := range []string{ticketJob, request} {
		var retryID, status string
		if err := db.QueryRowContext(ctx, "SELECT JSON_UNQUOTE(JSON_EXTRACT(after_json,'$.retryJobId')),JSON_UNQUOTE(JSON_EXTRACT(after_json,'$.status')) FROM admin_audit_log WHERE object_type='EMAIL_JOB' AND object_id=? AND action='RETRY' ORDER BY id DESC LIMIT 1", id).Scan(&retryID, &status); err != nil {
			t.Fatal(err)
		}
		if id == ticketJob && (retryID != ticketJob || status != "PENDING") || id == request && (retryID != replacement || status != "SUPERSEDED") {
			t.Fatal("wrong retry audit")
		}
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case mail := <-mails:
			if mail.to != buyer {
				t.Fatalf("unexpected local SMTP recipient: %s", mail.to)
			}
			for _, kind := range []string{"E-ticket", "Pulihkan"} {
				if strings.Contains(mail.subject, kind) {
					seen[kind] = true
				}
			}
			t.Logf("local SMTP captured %s", mail.subject)
		case <-time.After(10 * time.Second):
			t.Fatal("SMTP sink did not receive both messages")
		}
	}
	if len(seen) != 2 {
		t.Fatalf("SMTP did not capture both ticket and recovery email: %v", seen)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_queue WHERE id IN (?,?) AND status='SENT'", ticketJob, replacement).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not persist SENT")
		}
		time.Sleep(25 * time.Millisecond)
	}
	stop()
	<-workerDone
	after, err := metrics.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.FailedEmailJobs != 0 || len(list()) != 1 {
		t.Fatalf("after retry: metric=%d list=%d, want 0 and 1", after.FailedEmailJobs, len(list()))
	}
	// The failure list can contain one excluded recovery failure while monitoring is zero.
	// Rate limiting remains authoritative when a new failure occurs within the cooldown.
	execSQL("UPDATE email_queue SET status='FAILED',updated_at=UTC_TIMESTAMP(6) WHERE id=?", ticketJob)
	if res := call("POST", "email-jobs/"+ticketJob+"/retry"); res.Code != 429 {
		t.Fatalf("cooldown retry: %d %s", res.Code, res.Body.String())
	}
	execSQL("UPDATE email_queue SET status='SENT',updated_at=UTC_TIMESTAMP(6) WHERE id=?", ticketJob)
	t.Log("verified: FAILED scope, masked/empty recipient, canRetry denial, same-ID/replacement RETRY audits, local SMTP delivery, persisted SENT, monitoring 1→0 with 1 excluded failure, duplicate and rate-limit rejection")
}
