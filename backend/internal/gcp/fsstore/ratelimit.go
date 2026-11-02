package fsstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

// RateLimiter đếm thao tác ghi theo người và theo phút trong users/{uid}/rate/{phút},
// dùng chung cho mọi instance của mimi-api (vấn đề 79). TTL policy trên expire_at xóa
// tài liệu cũ, nên mỗi người chỉ giữ vài tài liệu.
type RateLimiter struct{ s *Store }

func NewRateLimiter(s *Store) *RateLimiter { return &RateLimiter{s: s} }

func (l *RateLimiter) Allow(ctx context.Context, uid string, limit int, now time.Time) (bool, error) {
	minute := now.UTC().Truncate(time.Minute)
	ref := l.s.user(uid).Collection("rate").Doc(minute.Format("200601021504"))
	allowed := false
	err := l.s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		allowed = false
		var count int64
		snap, err := tx.Get(ref)
		switch {
		case err == nil:
			count, _ = snap.Data()["count"].(int64)
		case !isNotFound(err):
			return err
		}
		if count >= int64(limit) {
			return nil // đã đủ lượt: không ghi gì thêm
		}
		allowed = true
		return tx.Set(ref, map[string]any{"count": count + 1, "expire_at": minute.Add(10 * time.Minute)})
	})
	if err != nil {
		return false, fmt.Errorf("rate limit: %w", err)
	}
	return allowed, nil
}
