package staffauth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
)

const sessionDuration = 8 * time.Hour
const passwordIterations = 600_000
const passwordHashBytes = 32

var (
	ErrInvalidInput   = errors.New("invalid staff input")
	ErrCredentials    = errors.New("invalid staff credentials")
	ErrUnauthorized   = errors.New("staff authentication required")
	ErrForbidden      = errors.New("staff permission denied")
	ErrNotFound       = errors.New("staff account not found")
	ErrDuplicateEmail = errors.New("staff email already exists")
	ErrRateLimited    = errors.New("staff login rate limited")
	ErrAdminExists    = errors.New("an administrator already exists")
)

type Assignment struct {
	EventID string `json:"eventId"`
	Gate    string `json:"gate"`
}

type Staff struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Role        string       `json:"role"`
	Active      bool         `json:"active"`
	Assignments []Assignment `json:"assignments"`
}

type Session struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
	Staff       Staff  `json:"staff"`
}

type Service struct {
	db      *sql.DB
	limiter *loginLimiter
}

func New(db *sql.DB) *Service { return &Service{db: db, limiter: newLoginLimiter()} }

func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func ValidateIdentity(name, email string) (string, string, error) {
	name = strings.TrimSpace(name)
	email = NormalizeEmail(email)
	address, err := mail.ParseAddress(email)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 80 || len(email) > 254 || err != nil || address.Address != email || !strings.Contains(strings.SplitN(email, "@", 2)[1], ".") {
		return "", "", ErrInvalidInput
	}
	return name, email, nil
}

func ValidatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return ErrInvalidInput
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate staff password salt: %w", err)
	}
	derived, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordHashBytes)
	if err != nil {
		return "", fmt.Errorf("derive staff password hash: %w", err)
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}

func verifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations != passwordIterations {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) != passwordHashBytes {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func tokenHash(token string) []byte { hash := sha256.Sum256([]byte(token)); return hash[:] }

func newSessionToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate staff session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

type Principal struct{ Staff }

func (s *Service) Login(ctx context.Context, email, password, remoteAddr string) (Session, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()
	if !s.limiter.Allow(remoteAddr, email, now) {
		return Session{}, ErrRateLimited
	}
	var user Staff
	var passwordHash string
	err := s.db.QueryRowContext(ctx, "SELECT id, name, email, role, password_hash, active FROM staff_users WHERE email = ?", email).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &passwordHash, &user.Active)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = pbkdf2.Key(sha256.New, password, []byte("staff-login-dummy"), passwordIterations, passwordHashBytes)
		return Session{}, s.limiter.Failure(remoteAddr, email, now, ErrCredentials)
	}
	if err != nil {
		return Session{}, fmt.Errorf("find staff login: %w", err)
	}
	if !verifyPassword(password, passwordHash) || !user.Active {
		return Session{}, s.limiter.Failure(remoteAddr, email, now, ErrCredentials)
	}
	s.limiter.Success(email, now)
	return s.createSession(ctx, user, passwordHash)
}

func (s *Service) createSession(ctx context.Context, user Staff, expectedPasswordHash string) (Session, error) {
	token, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().UTC().Add(sessionDuration)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Session{}, fmt.Errorf("begin staff session: %w", err)
	}
	defer tx.Rollback()
	var currentHash string
	var active bool
	err = tx.QueryRowContext(ctx, "SELECT password_hash, active FROM staff_users WHERE id = ? FOR UPDATE", user.ID).Scan(&currentHash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock staff for session: %w", err)
	}
	if !active || currentHash != expectedPasswordHash {
		return Session{}, ErrCredentials
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM staff_sessions WHERE expires_at <= UTC_TIMESTAMP(6)"); err != nil {
		return Session{}, fmt.Errorf("delete expired staff sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO staff_sessions (token_hash, staff_id, expires_at, created_at) VALUES (?, ?, ?, ?)", tokenHash(token), user.ID, expiresAt, time.Now().UTC()); err != nil {
		return Session{}, fmt.Errorf("insert staff session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Session{}, fmt.Errorf("commit staff session: %w", err)
	}
	user.Assignments, err = s.assignments(ctx, user.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, ExpiresAt: expiresAt.Format(time.RFC3339), Staff: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if len(token) != 43 {
		return Principal{}, ErrUnauthorized
	}
	var user Staff
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.name, u.email, u.role, u.active FROM staff_sessions s
		JOIN staff_users u ON u.id = s.staff_id WHERE s.token_hash = ? AND s.expires_at > UTC_TIMESTAMP(6) AND u.active = TRUE`, tokenHash(token)).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, fmt.Errorf("authenticate staff session: %w", err)
	}
	return Principal{Staff: user}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return ErrUnauthorized
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM staff_sessions WHERE token_hash = ?", tokenHash(token)); err != nil {
		return fmt.Errorf("delete staff session: %w", err)
	}
	return nil
}

func (s *Service) Current(ctx context.Context, principal Principal) (Staff, error) {
	staff := principal.Staff
	assignments, err := s.assignments(ctx, staff.ID)
	if err != nil {
		return Staff{}, err
	}
	staff.Assignments = assignments
	return staff, nil
}

func (s *Service) assignments(ctx context.Context, staffID string) ([]Assignment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT event_id, gate FROM staff_assignments WHERE staff_id = ? ORDER BY event_id, gate", staffID)
	if err != nil {
		return nil, fmt.Errorf("list staff assignments: %w", err)
	}
	defer rows.Close()
	items := []Assignment{}
	for rows.Next() {
		var item Assignment
		if err := rows.Scan(&item.EventID, &item.Gate); err != nil {
			return nil, fmt.Errorf("scan staff assignment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate staff assignments: %w", err)
	}
	return items, nil
}

func (s *Service) AuthorizeGate(ctx context.Context, principal Principal, eventID, gate string) error {
	if principal.Role != "STAFF" {
		return ErrForbidden
	}
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM staff_assignments WHERE staff_id = ? AND event_id = ? AND gate = ?", principal.ID, eventID, gate).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("check staff gate assignment: %w", err)
	}
	return nil
}

func (s *Service) CreateStaff(ctx context.Context, name, email, password string, assignments []Assignment) (Staff, error) {
	name, email, err := ValidateIdentity(name, email)
	if err != nil {
		return Staff{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Staff{}, err
	}
	assignments = normalizeAssignments(assignments)
	passwordHash, err := hashPassword(password)
	if err != nil {
		return Staff{}, err
	}
	id, err := newID()
	if err != nil {
		return Staff{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Staff{}, fmt.Errorf("begin create staff: %w", err)
	}
	defer tx.Rollback()
	if err := validateAssignments(ctx, tx, assignments); err != nil {
		return Staff{}, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, "INSERT INTO staff_users (id, name, email, password_hash, role, active, created_at, updated_at) VALUES (?, ?, ?, ?, 'STAFF', TRUE, ?, ?)", id, name, email, passwordHash, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return Staff{}, ErrDuplicateEmail
		}
		return Staff{}, fmt.Errorf("insert staff: %w", err)
	}
	if err := replaceAssignments(ctx, tx, id, assignments, now); err != nil {
		return Staff{}, err
	}
	if err := tx.Commit(); err != nil {
		return Staff{}, fmt.Errorf("commit create staff: %w", err)
	}
	return Staff{ID: id, Name: name, Email: email, Role: "STAFF", Active: true, Assignments: assignments}, nil
}

func (s *Service) ListStaff(ctx context.Context) ([]Staff, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, email, role, active FROM staff_users ORDER BY name, id")
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	all := []Staff{}
	for rows.Next() {
		var row Staff
		if err := rows.Scan(&row.ID, &row.Name, &row.Email, &row.Role, &row.Active); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan staff: %w", err)
		}
		all = append(all, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate staff: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close staff: %w", err)
	}
	result := make([]Staff, 0, len(all))
	for _, row := range all {
		row.Assignments, err = s.assignments(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *Service) UpdateStaff(ctx context.Context, staffID string, name *string, active *bool) error {
	if name == nil && active == nil {
		return ErrInvalidInput
	}
	if name != nil {
		*name = strings.TrimSpace(*name)
		if utf8.RuneCountInString(*name) < 2 || utf8.RuneCountInString(*name) > 80 {
			return ErrInvalidInput
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staff update: %w", err)
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRowContext(ctx, "SELECT role FROM staff_users WHERE id = ? FOR UPDATE", staffID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock staff update: %w", err)
	}
	if role != "STAFF" {
		return ErrForbidden
	}
	if name != nil {
		if _, err := tx.ExecContext(ctx, "UPDATE staff_users SET name = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?", *name, staffID); err != nil {
			return fmt.Errorf("update staff name: %w", err)
		}
	}
	if active != nil {
		if _, err := tx.ExecContext(ctx, "UPDATE staff_users SET active = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?", *active, staffID); err != nil {
			return fmt.Errorf("update staff active: %w", err)
		}
		if !*active {
			if _, err := tx.ExecContext(ctx, "DELETE FROM staff_sessions WHERE staff_id = ?", staffID); err != nil {
				return fmt.Errorf("revoke inactive staff sessions: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff update: %w", err)
	}
	return nil
}

func (s *Service) ReplaceAssignments(ctx context.Context, staffID string, assignments []Assignment) error {
	if assignments == nil {
		return ErrInvalidInput
	}
	assignments = normalizeAssignments(assignments)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staff assignments: %w", err)
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRowContext(ctx, "SELECT role FROM staff_users WHERE id = ? FOR UPDATE", staffID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock staff assignments: %w", err)
	}
	if role != "STAFF" {
		return ErrForbidden
	}
	if err := validateAssignments(ctx, tx, assignments); err != nil {
		return err
	}
	if err := replaceAssignments(ctx, tx, staffID, assignments, time.Now().UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff assignments: %w", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, staffID, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staff password reset: %w", err)
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRowContext(ctx, "SELECT role FROM staff_users WHERE id = ? FOR UPDATE", staffID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock staff password reset: %w", err)
	}
	if role != "STAFF" {
		return ErrForbidden
	}
	if _, err := tx.ExecContext(ctx, "UPDATE staff_users SET password_hash = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?", passwordHash, staffID); err != nil {
		return fmt.Errorf("update staff password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM staff_sessions WHERE staff_id = ?", staffID); err != nil {
		return fmt.Errorf("revoke reset staff sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff password reset: %w", err)
	}
	return nil
}

type assignmentTx interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func validateAssignments(ctx context.Context, tx assignmentTx, assignments []Assignment) error {
	seen := map[Assignment]bool{}
	for _, assignment := range assignments {
		assignment.EventID = strings.TrimSpace(assignment.EventID)
		assignment.Gate = strings.TrimSpace(assignment.Gate)
		if assignment.EventID == "" || assignment.Gate == "" || len(assignment.EventID) > 64 || len(assignment.Gate) > 100 || seen[assignment] {
			return ErrInvalidInput
		}
		seen[assignment] = true
		var exists int
		err := tx.QueryRowContext(ctx, "SELECT 1 FROM ticket_tiers WHERE event_id = ? AND gate = ? LIMIT 1", assignment.EventID, assignment.Gate).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidInput
		}
		if err != nil {
			return fmt.Errorf("validate staff gate: %w", err)
		}
	}
	return nil
}

func normalizeAssignments(assignments []Assignment) []Assignment {
	result := make([]Assignment, len(assignments))
	for i, assignment := range assignments {
		result[i] = Assignment{EventID: strings.TrimSpace(assignment.EventID), Gate: strings.TrimSpace(assignment.Gate)}
	}
	return result
}

func replaceAssignments(ctx context.Context, tx assignmentTx, staffID string, assignments []Assignment, now time.Time) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM staff_assignments WHERE staff_id = ?", staffID); err != nil {
		return fmt.Errorf("delete staff assignments: %w", err)
	}
	for _, assignment := range assignments {
		if _, err := tx.ExecContext(ctx, "INSERT INTO staff_assignments (staff_id, event_id, gate, created_at) VALUES (?, ?, ?, ?)", staffID, strings.TrimSpace(assignment.EventID), strings.TrimSpace(assignment.Gate), now); err != nil {
			return fmt.Errorf("insert staff assignment: %w", err)
		}
	}
	return nil
}

func (s *Service) BootstrapAdmin(ctx context.Context, name, email, password string) error {
	name, email, err := ValidateIdentity(name, email)
	if err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return err
	}
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect bootstrap admin: %w", err)
	}
	defer connection.Close()
	var acquired sql.NullInt64
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK('ticket-online-bootstrap-admin', 30)").Scan(&acquired); err != nil {
		return fmt.Errorf("acquire bootstrap admin lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("could not acquire bootstrap admin lock")
	}
	defer connection.ExecContext(context.Background(), "SELECT RELEASE_LOCK('ticket-online-bootstrap-admin')")
	var count int
	if err := connection.QueryRowContext(ctx, "SELECT COUNT(*) FROM staff_users WHERE role = 'ADMIN'").Scan(&count); err != nil {
		return fmt.Errorf("check bootstrap admin: %w", err)
	}
	if count != 0 {
		return ErrAdminExists
	}
	id, err := newID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = connection.ExecContext(ctx, "INSERT INTO staff_users (id, name, email, password_hash, role, active, created_at, updated_at) VALUES (?, ?, ?, ?, 'ADMIN', TRUE, ?, ?)", id, name, email, passwordHash, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("insert bootstrap admin: %w", err)
	}
	return nil
}

func newID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate staff id: %w", err)
	}
	return fmt.Sprintf("%x", data[:]), nil
}

type loginLimiter struct {
	mu      sync.Mutex
	windows map[string]limitWindow
}
type limitWindow struct {
	count int
	until time.Time
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{windows: make(map[string]limitWindow)} }

func (l *loginLimiter) Allow(remote, email string, now time.Time) bool {
	peer := sourceIP(remote)
	if peer == "" {
		peer = "unknown"
	}
	ipKey, emailKey := "ip:"+peer, "email:"+email
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	ip := l.windows[ipKey]
	if now.Before(ip.until) && ip.count >= 30 {
		return false
	}
	failure := l.windows[emailKey]
	if now.Before(failure.until) && failure.count >= 5 {
		return false
	}
	if len(l.windows) >= 10000 && l.windows[ipKey].until.IsZero() {
		return false
	}
	if !now.Before(ip.until) {
		ip = limitWindow{until: now.Add(time.Minute)}
	}
	ip.count++
	l.windows[ipKey] = ip
	return true
}

func (l *loginLimiter) Failure(remote, email string, now time.Time, result error) error {
	peer := sourceIP(remote)
	key := "email:" + NormalizeEmail(email)
	if peer == "" {
		peer = "unknown"
	}
	if email == "" {
		key += "@" + peer
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	if _, exists := l.windows[key]; !exists && len(l.windows) >= 10000 {
		return ErrRateLimited
	}
	window := l.windows[key]
	if !now.Before(window.until) {
		window = limitWindow{until: now.Add(15 * time.Minute)}
	}
	window.count++
	l.windows[key] = window
	if window.count >= 5 {
		return ErrRateLimited
	}
	return result
}

func (l *loginLimiter) Success(email string, now time.Time) {
	l.mu.Lock()
	delete(l.windows, "email:"+NormalizeEmail(email))
	l.mu.Unlock()
}

func (l *loginLimiter) prune(now time.Time) {
	for key, window := range l.windows {
		if !now.Before(window.until) {
			delete(l.windows, key)
		}
	}
}

func sourceIP(remote string) string {
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}
