package fsstore

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// ---------- Phiên bản dữ liệu (A4) ----------
//
// Mỗi tài liệu users/{uid} có một số nguyên schema_version: dữ liệu của người đó đang ở
// phiên bản nào. Chỉ một ô số cho mỗi người dùng, không thêm tài liệu nào, nên dữ liệu
// không phình ra theo thời gian.
//
// Khi đổi cách lưu (ví dụ B3 gắn plan_id cho từng dòng hóa đơn), thêm một bước vào
// migrations với Version tiếp theo. Bước chạy:
//   - lúc người dùng đăng nhập mà tài liệu còn ở phiên bản cũ (LoadPrincipal), và
//   - bằng job "migrate" chạy qua mọi người dùng, cho người lâu không đăng nhập.
// Mỗi bước phải chạy lại được nhiều lần mà không hỏng (idempotent), vì hai đường trên có
// thể chạy cùng lúc. Code đọc vẫn phải hiểu dữ liệu cũ cho tới khi mọi người đã lên bản
// mới, để quay lại bản cũ (README, mục Quay lại bản cũ) không làm hỏng dữ liệu.

type migration struct {
	Version int
	Name    string
	Run     func(ctx context.Context, s *Store, uid string) error // nil: chỉ đánh dấu phiên bản
}

// migrations theo thứ tự Version tăng dần, liên tiếp từ 1.
var migrations = []migration{
	// 1: mốc đầu. Dữ liệu có từ trước khi có schema_version được coi là phiên bản 1.
	{Version: 1, Name: "baseline"},
}

// SchemaVersion là phiên bản dữ liệu mà code này ghi ra.
var SchemaVersion = migrations[len(migrations)-1].Version

func pendingMigrations(from int) []migration {
	var out []migration
	for _, m := range migrations {
		if m.Version > from {
			out = append(out, m)
		}
	}
	return out
}

// migrateUser chạy các bước còn thiếu rồi ghi schema_version. Không bao giờ hạ phiên bản:
// bản code cũ (sau khi quay lại) gặp dữ liệu mới hơn thì để nguyên.
func (s *Store) migrateUser(ctx context.Context, uid string, from int) error {
	steps := pendingMigrations(from)
	if len(steps) == 0 {
		return nil
	}
	for _, m := range steps {
		if m.Run != nil {
			if err := m.Run(ctx, s, uid); err != nil {
				return fmt.Errorf("migrate %s to v%d (%s): %w", uid, m.Version, m.Name, err)
			}
		}
	}
	target := steps[len(steps)-1].Version
	ref := s.user(uid)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if err != nil {
			return err
		}
		if cur := asInt64(snap.Data()["schema_version"]); cur >= int64(target) {
			return nil
		}
		return tx.Update(ref, []firestore.Update{{Path: "schema_version", Value: int64(target)}})
	})
}

// MigrateAll là job "migrate": đưa mọi người dùng lên SchemaVersion. Trả số người đã nâng.
func (s *Store) MigrateAll(ctx context.Context, log *slog.Logger) (int, error) {
	it := s.c.Collection("users").Select("schema_version").Documents(ctx)
	defer it.Stop()
	n := 0
	for {
		d, err := it.Next()
		if err == iterator.Done {
			return n, nil
		}
		if err != nil {
			return n, fmt.Errorf("migrate list: %w", err)
		}
		from := int(asInt64(d.Data()["schema_version"]))
		if from >= SchemaVersion {
			continue
		}
		if err := s.migrateUser(ctx, d.Ref.ID, from); err != nil {
			return n, err
		}
		n++
		if n%100 == 0 {
			log.InfoContext(ctx, "migrate progress", "users", n)
		}
	}
}
