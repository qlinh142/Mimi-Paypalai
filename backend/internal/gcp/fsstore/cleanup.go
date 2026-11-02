package fsstore

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// EventRetention: thông báo trong app giữ 90 ngày. Màn Thông báo chỉ hiện 30 ngày gần nhất.
const EventRetention = 90 * 24 * time.Hour

// CleanupResult đếm số thứ đã dọn trong một lượt job cleanup.
type CleanupResult struct {
	Invites int // mã mời hết hạn đã gỡ khỏi nhà
	Events  int // thông báo quá 90 ngày đã xóa
}

// Cleanup là job hằng ngày (ADR-001): gỡ mã mời hết hạn và xóa thông báo cũ.
// Tài liệu tạm có expire_at (drafts, quota, page_cache, idempotency, rate, events mới) do
// TTL policy của Firestore xóa; job này lo phần TTL không phủ: mã mời nằm ngay trên tài liệu
// nhà (không xóa cả tài liệu được) và thông báo ghi trước khi có expire_at.
// Chạy lại nhiều lần không sao: mỗi bước chỉ chạm vào thứ đã hết hạn.
func (s *Store) Cleanup(ctx context.Context, now time.Time, log *slog.Logger) (CleanupResult, error) {
	var res CleanupResult
	n, err := s.clearExpiredInvites(ctx, now)
	res.Invites = n
	if err != nil {
		return res, err
	}
	n, err = s.deleteOldEvents(ctx, now.Add(-EventRetention), log)
	res.Events = n
	return res, err
}

func (s *Store) clearExpiredInvites(ctx context.Context, now time.Time) (int, error) {
	docs, err := s.c.Collection("households").Where("invite_expires_at", "<", now).Documents(ctx).GetAll()
	if err != nil {
		return 0, fmt.Errorf("cleanup invites: %w", err)
	}
	for _, d := range docs {
		_, err := d.Ref.Update(ctx, []firestore.Update{
			{Path: "invite_code", Value: firestore.Delete},
			{Path: "invite_expires_at", Value: firestore.Delete},
		}, firestore.LastUpdateTime(d.UpdateTime)) // chủ nhà vừa tạo mã mới thì bỏ qua
		if err != nil && !isPrecondition(err) {
			return 0, fmt.Errorf("cleanup invite %s: %w", d.Ref.ID, err)
		}
	}
	return len(docs), nil
}

func (s *Store) deleteOldEvents(ctx context.Context, before time.Time, log *slog.Logger) (int, error) {
	users := s.c.Collection("users").Select().Documents(ctx)
	defer users.Stop()
	bw := s.c.BulkWriter(ctx)
	var jobs []*firestore.BulkWriterJob
	for {
		u, err := users.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bw.End()
			return 0, fmt.Errorf("cleanup users: %w", err)
		}
		old, err := u.Ref.Collection("events").Where("at", "<", before).Select().Documents(ctx).GetAll()
		if err != nil {
			bw.End()
			return 0, fmt.Errorf("cleanup events %s: %w", u.Ref.ID, err)
		}
		for _, d := range old {
			j, err := bw.Delete(d.Ref)
			if err != nil {
				bw.End()
				return 0, err
			}
			jobs = append(jobs, j)
		}
	}
	bw.End()
	failed := 0
	for _, j := range jobs {
		if _, err := j.Results(); err != nil {
			failed++
		}
	}
	if failed > 0 {
		log.WarnContext(ctx, "cleanup events: some deletes failed", "failed", failed)
		return len(jobs) - failed, fmt.Errorf("cleanup events: %d deletes failed", failed)
	}
	return len(jobs), nil
}
