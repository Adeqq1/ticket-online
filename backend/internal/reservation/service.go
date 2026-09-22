package reservation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrInvalidRequest       = errors.New("invalid request")
	ErrEventNotFound        = errors.New("event not found")
	ErrTierNotFound         = errors.New("tier not found")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrOrderLimitExceeded   = errors.New("order limit exceeded")
	ErrIdempotencyConflict  = errors.New("idempotency conflict")
	ErrReservationNotFound  = errors.New("reservation not found")
	ErrReservationExpired   = errors.New("reservation expired")
	ErrReservationCancelled = errors.New("reservation cancelled")
)

type Request struct {
	EventID string        `json:"eventId"`
	Items   []ItemRequest `json:"items"`
}

type ItemRequest struct {
	TierID   string `json:"tierId"`
	Quantity uint64 `json:"quantity"`
}

type canonicalRequest struct {
	EventID string        `json:"eventId"`
	Items   []ItemRequest `json:"items"`
}

func ValidateRequest(request Request) error {
	request.EventID = strings.TrimSpace(request.EventID)
	if request.EventID == "" || len(request.Items) == 0 || len(request.Items) > 10 {
		return ErrInvalidRequest
	}
	seen := make(map[string]struct{}, len(request.Items))
	for _, item := range request.Items {
		id := strings.TrimSpace(item.TierID)
		if id == "" || item.Quantity == 0 || item.Quantity > uint64(^uint32(0)) {
			return ErrInvalidRequest
		}
		if _, ok := seen[id]; ok {
			return ErrInvalidRequest
		}
		seen[id] = struct{}{}
	}
	return nil
}

func NormalizeRequest(request Request) Request {
	request.EventID = strings.TrimSpace(request.EventID)
	for index := range request.Items {
		request.Items[index].TierID = strings.TrimSpace(request.Items[index].TierID)
	}
	return request
}

func RequestHash(request Request) (string, error) {
	canonical := canonicalRequest{EventID: strings.TrimSpace(request.EventID), Items: make([]ItemRequest, len(request.Items))}
	for index, item := range request.Items {
		canonical.Items[index] = ItemRequest{TierID: strings.TrimSpace(item.TierID), Quantity: item.Quantity}
	}
	sort.Slice(canonical.Items, func(i, j int) bool { return canonical.Items[i].TierID < canonical.Items[j].TierID })
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal request hash: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
