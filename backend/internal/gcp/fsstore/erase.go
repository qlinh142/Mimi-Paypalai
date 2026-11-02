package fsstore

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// EraseUser xóa đệ quy users/{uid} và mọi collection con (B10).
func (s *Store) EraseUser(ctx context.Context, uid string) error {
	return s.eraseTree(ctx, s.user(uid))
}

// EraseHousehold xóa đệ quy households/{hid}: vòng mua hộ, mã mời.
func (s *Store) EraseHousehold(ctx context.Context, hid string) error {
	return s.eraseTree(ctx, s.household(hid))
}

// eraseTree xóa một tài liệu và mọi thứ bên dưới, con trước cha, bằng BulkWriter. SDK Go
// chưa có RecursiveDelete. Tài liệu chỉ có collection con (không có dữ liệu) vẫn được duyệt
// nhờ DocumentRefs. Gọi lại khi đã xóa hết thì không làm gì.
func (s *Store) eraseTree(ctx context.Context, root *firestore.DocumentRef) error {
	bw := s.c.BulkWriter(ctx)
	var jobs []*firestore.BulkWriterJob
	var walk func(ref *firestore.DocumentRef) error
	walk = func(ref *firestore.DocumentRef) error {
		cols := ref.Collections(ctx)
		for {
			col, err := cols.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return err
			}
			docs := col.DocumentRefs(ctx)
			for {
				d, err := docs.Next()
				if errors.Is(err, iterator.Done) {
					break
				}
				if err != nil {
					return err
				}
				if err := walk(d); err != nil {
					return err
				}
			}
		}
		j, err := bw.Delete(ref)
		if err != nil {
			return err
		}
		jobs = append(jobs, j)
		return nil
	}
	err := walk(root)
	bw.End()
	if err != nil {
		return fmt.Errorf("erase %s: %w", root.Path, err)
	}
	for _, j := range jobs {
		if _, err := j.Results(); err != nil {
			return fmt.Errorf("erase %s: %w", root.Path, err)
		}
	}
	return nil
}
