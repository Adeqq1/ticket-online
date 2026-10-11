package posters

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestUploadBodyDeadlineReleasesSlots(t *testing.T) {
	for _, kind := range []string{"multipart headers", "file", "chunked file", "epilogue", "chunked epilogue"} {
		t.Run(kind, func(t *testing.T) {
			var writes atomic.Int32
			h := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				writes.Add(1)
				io.Copy(io.Discard, r.Body)
				w.WriteHeader(200)
			})
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
				defer cancel()
				h.Upload(w, r.WithContext(ctx))
			}))
			server.Config.ReadTimeout = 2 * time.Second
			server.Config.WriteTimeout = 3 * time.Second
			server.Start()
			defer server.Close()
			req := uploadRequest(t, picture(t, "png"), "poster.png", 1)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			partial := body
			if kind == "multipart headers" {
				partial = body[:20]
			} else if strings.Contains(kind, "file") {
				partial = body[:bytes.Index(body, []byte("\r\n\r\n"))+4+10]
			}
			started := time.Now()
			var connections []net.Conn
			for range 2 {
				conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
				headers := fmt.Sprintf("POST /api/v1/admin/posters HTTP/1.1\r\nHost: localhost\r\nAuthorization: %s\r\nContent-Type: %s\r\n", req.Header.Get("Authorization"), req.Header.Get("Content-Type"))
				if strings.HasPrefix(kind, "chunked") {
					headers += "Transfer-Encoding: chunked\r\n\r\n"
					headers += fmt.Sprintf("%x\r\n", len(partial))
				} else {
					headers += fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)+100)
				}
				if _, err = io.WriteString(conn, headers); err != nil {
					t.Fatal(err)
				}
				if _, err = conn.Write(partial); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(kind, "chunked") {
					if _, err = io.WriteString(conn, "\r\n"); err != nil {
						t.Fatal(err)
					}
				}
				connections = append(connections, conn)
			}
			until := time.Now().Add(time.Second)
			for len(h.slots) != 2 && time.Now().Before(until) {
				time.Sleep(time.Millisecond)
			}
			if len(h.slots) != 2 {
				t.Fatal("slow uploads did not acquire both slots")
			}
			call := func(want int) {
				t.Helper()
				r := uploadRequest(t, picture(t, "png"), "poster.png", 1)
				r.URL.Scheme = "http"
				r.URL.Host = server.Listener.Addr().String()
				r.RequestURI = ""
				response, err := server.Client().Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != want {
					data, _ := io.ReadAll(response.Body)
					t.Fatalf("status %d want %d: %s", response.StatusCode, want, data)
				}
			}
			call(429)
			for _, conn := range connections {
				response, err := http.ReadResponse(bufio.NewReader(conn), req)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != 504 || !bytes.Contains(data, []byte("POSTER_TIMEOUT")) {
					t.Fatalf("slow body: %d %s", response.StatusCode, data)
				}
			}
			if elapsed := time.Since(started); elapsed > 1500*time.Millisecond {
				t.Fatalf("body waited beyond operation budget: %s", elapsed)
			}
			if writes.Load() != 0 {
				t.Fatal("timed out body reached S3")
			}
			call(201)
			if writes.Load() != 1 || len(h.slots) != 0 {
				t.Fatalf("slots not released: writes=%d slots=%d", writes.Load(), len(h.slots))
			}
		})
	}
}

func TestUploadRequiresReadDeadlineSupport(t *testing.T) {
	h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("S3 reached") })
	r := uploadRequest(t, picture(t, "png"), "poster.png", 1)
	body := &unreadableBody{}
	r.Body = body
	w := httptest.NewRecorder()
	h.Upload(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "POSTER_READ_DEADLINE_FAILED") || body.read || len(h.slots) != 0 {
		t.Fatalf("deadline failure: %d read=%v slots=%d", w.Code, body.read, len(h.slots))
	}
}

func TestMultipartTimeoutClassification(t *testing.T) {
	h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("S3 reached") })
	for _, err := range []error{context.DeadlineExceeded, fmt.Errorf("read socket: %w", &net.DNSError{IsTimeout: true})} {
		w := httptest.NewRecorder()
		h.multipartError(w, httptest.NewRequest("POST", "/", nil), err)
		if w.Code != 504 {
			t.Fatalf("%v: status %d", err, w.Code)
		}
	}
}

func TestExpiredUploadDoesNotDecodeOrStore(t *testing.T) {
	h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("S3 reached") })
	r := uploadRequest(t, []byte("invalid image"), "poster.png", 1)
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(-time.Second))
	defer cancel()
	w := newUploadRecorder()
	h.Upload(w, r.WithContext(ctx))
	if w.Code != 504 {
		t.Fatalf("expired operation continued to validation: %d %s", w.Code, w.Body.String())
	}
}

type trackedBody struct {
	reader *bytes.Reader
	reads  int
	closed bool
}

func (b *trackedBody) Read(p []byte) (int, error) { b.reads++; return b.reader.Read(p) }
func (b *trackedBody) Close() error               { b.closed = true; return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMediaExactETagSkipsBodyRead(t *testing.T) {
	data := picture(t, "png")
	name := strings.Repeat("a", 64) + ".png"
	etag := `"` + name + `"`
	for _, tc := range []struct {
		name          string
		headers       map[string][]string
		missing, read bool
		want          int
	}{
		{name: "ordinary GET", read: true, want: 200},
		{name: "exact ETag", headers: map[string][]string{"If-None-Match": {etag}}, want: 304},
		{name: "mismatch", headers: map[string][]string{"If-None-Match": {`"other"`}}, read: true, want: 200},
		{name: "missing object", headers: map[string][]string{"If-None-Match": {etag}}, missing: true, read: true, want: 404},
		{name: "If-Match takes precedence", headers: map[string][]string{"If-None-Match": {etag}, "If-Match": {`"other"`}}, read: true, want: 412},
		{name: "weak ETag fallback", headers: map[string][]string{"If-None-Match": {"W/" + etag}}, read: true, want: 304},
		{name: "ETag list fallback", headers: map[string][]string{"If-None-Match": {`"other", ` + etag}}, read: true, want: 304},
		{name: "multiple lines fallback", headers: map[string][]string{"If-None-Match": {etag, `"other"`}}, read: true, want: 304},
		{name: "range fallback", headers: map[string][]string{"If-None-Match": {etag}, "Range": {"bytes=0-9"}}, read: true, want: 304},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{reader: bytes.NewReader(data)}
			h := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("fixture transport reached") })
			h.client = s3.New(s3.Options{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				status := 200
				if tc.missing {
					status = 404
					body.reader = bytes.NewReader([]byte("<Error><Code>NoSuchKey</Code></Error>"))
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: body, Request: r}, nil
			})}})
			r := httptest.NewRequest("GET", "/api/v1/media/posters/"+name, nil)
			r.Header = http.Header(tc.headers)
			r.SetPathValue("filename", name)
			w := httptest.NewRecorder()
			h.Media(w, r)
			if w.Code != tc.want || (body.reads > 0) != tc.read || !body.closed {
				t.Fatalf("status=%d reads=%d closed=%v", w.Code, body.reads, body.closed)
			}
			if tc.want == 200 && !bytes.Equal(w.Body.Bytes(), data) {
				t.Fatal("ordinary GET lost bytes")
			}
			if tc.want == 304 && (w.Header().Get("ETag") != etag || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") || w.Body.Len() != 0) {
				t.Fatal(w.Header())
			}
		})
	}
}

func TestConditionalUploadDeduplicatesAndPreservesErrors(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	versions := 0
	loseResponse := false
	h := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "*" {
			t.Error("missing conditional PUT")
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if _, exists := objects[r.URL.Path]; exists {
			w.WriteHeader(412)
			io.WriteString(w, "<Error><Code>PreconditionFailed</Code></Error>")
			return
		}
		objects[r.URL.Path] = data
		versions++
		if loseResponse {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		w.WriteHeader(200)
	})
	upload := func(data []byte, want int) string {
		t.Helper()
		w := newUploadRecorder()
		h.Upload(w, uploadRequest(t, data, "poster.png", 1))
		if w.Code != want {
			t.Errorf("status=%d want=%d: %s", w.Code, want, w.Body.String())
		}
		return w.Body.String()
	}
	data := picture(t, "png")
	first := upload(data, 201)
	if next := upload(data, 201); next != first {
		t.Fatal("identical file changed URL")
	}
	other := picture(t, "jpeg")
	if next := upload(other, 201); next == first {
		t.Fatal("different file reused URL")
	}
	// Simultaneous uploads race for a previously unused key.
	fresh := append(bytes.Clone(data), 0)
	results := make(chan string, 2)
	for range 2 {
		go func() { results <- upload(fresh, 201) }()
	}
	if a, b := <-results, <-results; a != b {
		t.Fatal("concurrent upload changed URL")
	}
	mu.Lock()
	count := versions
	loseResponse = true
	mu.Unlock()
	if count != 3 {
		t.Fatalf("created %d versions, want 3", count)
	}
	lost := append(bytes.Clone(data), 1)
	upload(lost, 503)
	upload(lost, 201)
	mu.Lock()
	count = versions
	mu.Unlock()
	if count != 4 {
		t.Fatalf("retry created an extra version: %d", count)
	}
	for _, tc := range []struct {
		status int
		code   string
	}{{403, "AccessDenied"}, {404, "NoSuchBucket"}, {409, "ConditionalRequestConflict"}} {
		t.Run(tc.code, func(t *testing.T) {
			failing := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, "<Error><Code>%s</Code></Error>", tc.code)
			})
			w := newUploadRecorder()
			failing.Upload(w, uploadRequest(t, data, "poster.png", 1))
			if w.Code != 503 || strings.Contains(w.Body.String(), `"url"`) {
				t.Fatalf("storage error became success: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// A context may finish before the socket timeout error reaches multipartError.
func TestMultipartExpiredContext(t *testing.T) {
	h := fixture(t, func(http.ResponseWriter, *http.Request) {})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	w := httptest.NewRecorder()
	h.multipartError(w, httptest.NewRequest("POST", "/", nil).WithContext(ctx), errors.New("read failed"))
	if w.Code != 504 {
		t.Fatal(w.Code)
	}
}
