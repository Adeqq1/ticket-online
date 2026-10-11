package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/go-sql-driver/mysql"
)

var (
	ErrVoucher  = errors.New("voucher tidak tersedia, belum berlaku, tidak sesuai pembelian, atau kuota habis")
	ErrInvalid  = errors.New("pengaturan tidak valid")
	ErrConflict = errors.New("data telah berubah atau kuota lebih kecil dari penggunaan; muat ulang")
	codePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,31}$`)
)

type Settings struct {
	AdminFee uint64 `json:"adminFee"`
	Revision uint64 `json:"revision"`
}
type Voucher struct {
	Code            string `json:"code"`
	Kind            string `json:"kind"`
	Value           uint64 `json:"value"`
	StartsAt        string `json:"startsAt"`
	EndsAt          string `json:"endsAt"`
	Active          bool   `json:"active"`
	EventID         string `json:"eventId"`
	MinimumSubtotal uint64 `json:"minimumSubtotal"`
	Quota           uint64 `json:"quota"`
	Revision        uint64 `json:"revision"`
}
type Snapshot struct {
	Code  string `json:"code"`
	Kind  string `json:"kind"`
	Value uint64 `json:"value"`
}
type Quote struct {
	Subtotal uint64    `json:"subtotal"`
	AdminFee uint64    `json:"adminFee"`
	Discount uint64    `json:"discount"`
	Total    uint64    `json:"total"`
	Voucher  *Snapshot `json:"voucher"`
}
type Usage struct {
	Held     uint64 `json:"held"`
	Consumed uint64 `json:"consumed"`
	Released uint64 `json:"released"`
}
type ListedVoucher struct {
	Voucher
	Usage Usage `json:"usage"`
}
type Audit struct {
	Actor     string          `json:"actor"`
	ObjectID  string          `json:"objectId"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
	CreatedAt string          `json:"createdAt"`
}

func ValidCode(code string) bool { return codePattern.MatchString(code) }

func (v Voucher) Validate() error {
	start, a := time.Parse(time.RFC3339, v.StartsAt)
	end, b := time.Parse(time.RFC3339, v.EndsAt)
	if !ValidCode(v.Code) || (v.Kind != "PERCENT" && v.Kind != "FIXED") || v.Value == 0 || v.Value > 1000000000 || (v.Kind == "PERCENT" && v.Value > 100) || v.Quota == 0 || v.Quota > 1000000000 || v.MinimumSubtotal > 1000000000000 || a != nil || b != nil || !end.After(start) || start.Year() < 1970 || end.Year() > 9999 || len(v.EventID) > 64 {
		return ErrInvalid
	}
	return nil
}

// Discount uses integer rupiah and caps fixed discounts at the ticket subtotal.
func Discount(subtotal uint64, kind string, value uint64) uint64 {
	if kind == "PERCENT" {
		return subtotal/100*value + subtotal%100*value/100
	}
	return min(subtotal, value)
}

func readVoucher(ctx context.Context, tx *sql.Tx, code string, lock bool) (Voucher, error) {
	v := Voucher{}
	var start, end time.Time
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := tx.QueryRowContext(ctx, `SELECT code,kind,value,starts_at,ends_at,active,COALESCE(event_id,''),minimum_subtotal,quota,revision FROM vouchers WHERE code=?`+suffix, code).
		Scan(&v.Code, &v.Kind, &v.Value, &start, &end, &v.Active, &v.EventID, &v.MinimumSubtotal, &v.Quota, &v.Revision)
	v.StartsAt = start.UTC().Format(time.RFC3339Nano)
	v.EndsAt = end.UTC().Format(time.RFC3339Nano)
	return v, err
}

func usage(ctx context.Context, tx *sql.Tx, code string) (Usage, error) {
	var u Usage
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(NOT ov.consumed AND o.status='PENDING'),0), COALESCE(SUM(ov.consumed),0), COALESCE(SUM(NOT ov.consumed AND o.status<>'PENDING'),0)
	 FROM order_vouchers ov JOIN orders o ON o.id=ov.order_id WHERE ov.code=?`, code).Scan(&u.Held, &u.Consumed, &u.Released)
	return u, err
}

// Calculate locks the voucher for order creation; quotes never hold quota.
// Pending quota is released only after authoritative expiry/reconciliation changes order status.
func Calculate(ctx context.Context, tx *sql.Tx, eventID string, subtotal uint64, code string, lock bool) (Quote, error) {
	q := Quote{Subtotal: subtotal}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	if err := tx.QueryRowContext(ctx, "SELECT admin_fee FROM checkout_settings WHERE id=1"+suffix).Scan(&q.AdminFee); err != nil {
		return q, err
	}
	if subtotal > math.MaxUint64-q.AdminFee {
		return q, ErrInvalid
	}
	if code != "" {
		v, err := readVoucher(ctx, tx, code, lock)
		if errors.Is(err, sql.ErrNoRows) {
			return q, ErrVoucher
		}
		if err != nil {
			return q, err
		}
		start, _ := time.Parse(time.RFC3339, v.StartsAt)
		end, _ := time.Parse(time.RFC3339, v.EndsAt)
		now := time.Now().UTC()
		if !v.Active || now.Before(start) || !now.Before(end) || (v.EventID != "" && v.EventID != eventID) || subtotal < v.MinimumSubtotal {
			return q, ErrVoucher
		}
		u, err := usage(ctx, tx, code)
		if err != nil {
			return q, err
		}
		if u.Held+u.Consumed >= v.Quota {
			return q, ErrVoucher
		}
		q.Discount = Discount(subtotal, v.Kind, v.Value)
		q.Voucher = &Snapshot{v.Code, v.Kind, v.Value}
	}
	q.Total = subtotal + q.AdminFee - q.Discount
	// ponytail: payments require a positive amount; add a free-order issuance flow before allowing zero totals.
	if q.Total == 0 {
		return q, ErrVoucher
	}
	return q, nil
}

func LoadSnapshot(ctx context.Context, tx *sql.Tx, orderID string) (*Snapshot, error) {
	var v Snapshot
	err := tx.QueryRowContext(ctx, "SELECT code,kind,value FROM order_vouchers WHERE order_id=?", orderID).Scan(&v.Code, &v.Kind, &v.Value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

type Handler struct {
	db     *sql.DB
	auth   *staffauth.Service
	logger *slog.Logger
}

func NewHandler(db *sql.DB, auth *staffauth.Service, logger *slog.Logger) *Handler {
	return &Handler{db, auth, logger}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/pricing", h.list)
	mux.HandleFunc("PUT /api/v1/admin/pricing", h.saveSettings)
	mux.HandleFunc("POST /api/v1/admin/vouchers", h.saveVoucher)
	mux.HandleFunc("PUT /api/v1/admin/vouchers/{code}", h.saveVoucher)
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func (h *Handler) error(w http.ResponseWriter, r *http.Request, err error) {
	var my *mysql.MySQLError
	switch {
	case errors.Is(err, ErrInvalid), errors.As(err, &my) && (my.Number == 1452 || my.Number == 3819):
		fail(w, 422, "INVALID_REQUEST", "Periksa nominal, masa berlaku, cakupan event dan kuota.")
	case errors.Is(err, ErrConflict), errors.As(err, &my) && my.Number == 1062:
		fail(w, 409, "PRICING_CONFLICT", "Data berubah, kode sudah ada, atau kuota di bawah penggunaan. Muat ulang.")
	case errors.Is(err, sql.ErrNoRows):
		fail(w, 404, "NOT_FOUND", "Data tidak ditemukan.")
	default:
		h.logger.ErrorContext(r.Context(), "pricing error", "error", err)
		fail(w, 500, "INTERNAL_ERROR", "Pengaturan belum dapat diproses.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		fail(w, 400, "INVALID_JSON", "Data JSON tidak valid.")
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(w, 400, "INVALID_JSON", "Data JSON tidak valid.")
		return false
	}
	return true
}
func (h *Handler) admin(w http.ResponseWriter, r *http.Request) (staffauth.Principal, bool) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		fail(w, 401, "UNAUTHORIZED", "Sesi administrator diperlukan.")
		return staffauth.Principal{}, false
	}
	p, err := h.auth.Authenticate(r.Context(), token)
	if errors.Is(err, staffauth.ErrUnauthorized) {
		fail(w, 401, "UNAUTHORIZED", "Sesi sudah berakhir.")
		return p, false
	}
	if err != nil {
		h.error(w, r, err)
		return p, false
	}
	if p.Role != "ADMIN" {
		fail(w, 403, "FORBIDDEN", "Akses administrator diperlukan.")
		return p, false
	}
	return p, true
}
func audit(ctx context.Context, tx *sql.Tx, p staffauth.Principal, id string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_log(actor_id,actor_name,action,object_type,object_id,before_json,after_json,created_at) VALUES (?,?,'UPDATE','pricing',?,?,?,UTC_TIMESTAMP(6))`, p.ID, p.Name, id, b, a)
	return err
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.admin(w, r); !ok {
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		h.error(w, r, err)
		return
	}
	defer tx.Rollback()
	var s Settings
	if err = tx.QueryRowContext(r.Context(), "SELECT admin_fee,revision FROM checkout_settings WHERE id=1").Scan(&s.AdminFee, &s.Revision); err != nil {
		h.error(w, r, err)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT v.code,v.kind,v.value,v.starts_at,v.ends_at,v.active,COALESCE(v.event_id,''),v.minimum_subtotal,v.quota,v.revision,
	 COALESCE(SUM(NOT ov.consumed AND o.status='PENDING'),0),COALESCE(SUM(ov.consumed),0),COALESCE(SUM(NOT ov.consumed AND o.status<>'PENDING'),0)
	 FROM vouchers v LEFT JOIN order_vouchers ov ON ov.code=v.code LEFT JOIN orders o ON o.id=ov.order_id
	 GROUP BY v.code ORDER BY v.code`)
	if err != nil {
		h.error(w, r, err)
		return
	}
	vs := []ListedVoucher{}
	for rows.Next() {
		var v ListedVoucher
		var start, end time.Time
		if err = rows.Scan(&v.Code, &v.Kind, &v.Value, &start, &end, &v.Active, &v.EventID, &v.MinimumSubtotal, &v.Quota, &v.Revision, &v.Usage.Held, &v.Usage.Consumed, &v.Usage.Released); err != nil {
			break
		}
		v.StartsAt, v.EndsAt = start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)
		vs = append(vs, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		h.error(w, r, err)
		return
	}
	rows, err = tx.QueryContext(r.Context(), "SELECT actor_name,object_id,before_json,after_json,created_at FROM admin_audit_log WHERE object_type='pricing' ORDER BY id DESC LIMIT 100")
	if err != nil {
		h.error(w, r, err)
		return
	}
	history := []Audit{}
	for rows.Next() {
		var a Audit
		var at time.Time
		if err = rows.Scan(&a.Actor, &a.ObjectID, &a.Before, &a.After, &at); err != nil {
			break
		}
		a.CreatedAt = at.UTC().Format(time.RFC3339)
		history = append(history, a)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		h.error(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		h.error(w, r, err)
		return
	}
	respond(w, 200, map[string]any{"settings": s, "vouchers": vs, "history": history})
}
func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	p, ok := h.admin(w, r)
	if !ok {
		return
	}
	// Pointers distinguish a legitimate zero fee from a missing field.
	var input struct {
		AdminFee *uint64 `json:"adminFee"`
		Revision uint64  `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.AdminFee == nil || *input.AdminFee > 1000000000 || input.Revision == 0 {
		h.error(w, r, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.error(w, r, err)
		return
	}
	defer tx.Rollback()
	var old Settings
	err = tx.QueryRowContext(r.Context(), "SELECT admin_fee,revision FROM checkout_settings WHERE id=1 FOR UPDATE").Scan(&old.AdminFee, &old.Revision)
	if err != nil {
		h.error(w, r, err)
		return
	}
	if old.Revision != input.Revision {
		h.error(w, r, ErrConflict)
		return
	}
	next := Settings{*input.AdminFee, old.Revision + 1}
	_, err = tx.ExecContext(r.Context(), "UPDATE checkout_settings SET admin_fee=?,revision=? WHERE id=1", next.AdminFee, next.Revision)
	if err == nil {
		err = audit(r.Context(), tx, p, "admin-fee", old, next)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		h.error(w, r, err)
		return
	}
	respond(w, 200, next)
}
func (h *Handler) saveVoucher(w http.ResponseWriter, r *http.Request) {
	p, ok := h.admin(w, r)
	if !ok {
		return
	}
	// These fields have valid zero values, so require their explicit presence.
	var input struct {
		Voucher
		Active          *bool   `json:"active"`
		EventID         *string `json:"eventId"`
		MinimumSubtotal *uint64 `json:"minimumSubtotal"`
		Revision        *uint64 `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Active == nil || input.EventID == nil || input.MinimumSubtotal == nil || input.Revision == nil {
		h.error(w, r, ErrInvalid)
		return
	}
	v := input.Voucher
	v.Active, v.EventID, v.MinimumSubtotal, v.Revision = *input.Active, *input.EventID, *input.MinimumSubtotal, *input.Revision
	v.Code = strings.ToUpper(strings.TrimSpace(v.Code))
	if err := v.Validate(); err != nil {
		h.error(w, r, err)
		return
	}
	creating := r.Method == "POST"
	if (!creating && (r.PathValue("code") != v.Code || v.Revision == 0)) || (creating && v.Revision != 0) {
		h.error(w, r, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		h.error(w, r, err)
		return
	}
	defer tx.Rollback()
	var before any
	if !creating {
		old, e := readVoucher(r.Context(), tx, v.Code, true)
		if e != nil {
			h.error(w, r, e)
			return
		}
		before = old
		u, e := usage(r.Context(), tx, v.Code)
		if e != nil {
			h.error(w, r, e)
			return
		}
		if old.Revision != v.Revision || v.Quota < u.Held+u.Consumed {
			h.error(w, r, ErrConflict)
			return
		}
	}
	v.Revision++
	start, _ := time.Parse(time.RFC3339, v.StartsAt)
	end, _ := time.Parse(time.RFC3339, v.EndsAt)
	var event any
	if v.EventID != "" {
		event = v.EventID
	}
	if creating {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO vouchers(code,kind,value,starts_at,ends_at,active,event_id,minimum_subtotal,quota,revision) VALUES (?,?,?,?,?,?,?,?,?,?)`, v.Code, v.Kind, v.Value, start.UTC(), end.UTC(), v.Active, event, v.MinimumSubtotal, v.Quota, v.Revision)
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE vouchers SET kind=?,value=?,starts_at=?,ends_at=?,active=?,event_id=?,minimum_subtotal=?,quota=?,revision=? WHERE code=?`, v.Kind, v.Value, start.UTC(), end.UTC(), v.Active, event, v.MinimumSubtotal, v.Quota, v.Revision, v.Code)
	}
	if err == nil {
		err = audit(r.Context(), tx, p, v.Code, before, v)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		h.error(w, r, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	respond(w, status, v)
}
