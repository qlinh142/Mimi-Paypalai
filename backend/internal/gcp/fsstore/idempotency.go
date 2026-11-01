package fsstore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/idem"
)

// Idempotency hiện thực idem.Store ở users/{uid}/idempotency/{key}. Key do client sinh
// (UUID), đã được httpapi kiểm tra ký tự nên dùng thẳng làm ID tài liệu. expire_at để
// TTL policy tự xóa sau 24 giờ.
type Idempotency struct{ s *Store }

func NewIdempotency(s *Store) *Idempotency { return &Idempotency{s: s} }

type idemDoc struct {
	Fingerprint string    `firestore:"fp"`
	State       string    `firestore:"state"` // pending | done
	Status      int       `firestore:"status"`
	ContentType string    `firestore:"content_type"`
	Body        []byte    `firestore:"body"`
	StartedAt   time.Time `firestore:"started_at"`
	ExpireAt    time.Time `firestore:"expire_at"`
}

func (i *Idempotency) ref(uid, key string) *firestore.DocumentRef {
	return i.s.user(uid).Collection("idempotency").Doc(key)
}

func (i *Idempotency) Begin(ctx context.Context, uid, key, fp string, now time.Time) (*idem.Record, error) {
	ref := i.ref(uid, key)
	var out *idem.Record
	err := i.s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		out = nil
		snap, err := tx.Get(ref)
		if err != nil && !isNotFound(err) {
			return err
		}
		if err == nil {
			var d idemDoc
			if err := snap.DataTo(&d); err != nil {
				return err
			}
			fresh := now.Before(d.ExpireAt) && (d.State == "done" || now.Sub(d.StartedAt) <= idem.PendingStale)
			if fresh {
				switch {
				case d.Fingerprint != fp:
					return idem.ErrMismatch
				case d.State != "done":
					return idem.ErrInFlight
				}
				out = &idem.Record{Fingerprint: d.Fingerprint, Status: d.Status, ContentType: d.ContentType, Body: d.Body}
				return nil
			}
		}
		return tx.Set(ref, idemDoc{Fingerprint: fp, State: "pending", StartedAt: now, ExpireAt: now.Add(idem.TTL)})
	})
	return out, err
}

func (i *Idempotency) Finish(ctx context.Context, uid, key string, rec idem.Record, now time.Time) error {
	_, err := i.ref(uid, key).Set(ctx, idemDoc{
		Fingerprint: rec.Fingerprint, State: "done", Status: rec.Status, ContentType: rec.ContentType,
		Body: rec.Body, StartedAt: now, ExpireAt: now.Add(idem.TTL),
	})
	return err
}

func (i *Idempotency) Abort(ctx context.Context, uid, key string) error {
	_, err := i.ref(uid, key).Delete(ctx)
	return err
}
