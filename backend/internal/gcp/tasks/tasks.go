// Package tasks đẩy task vào Cloud Tasks qua REST API (B9). Dùng REST để không thêm thư viện
// gRPC mới; xác thực bằng Application Default Credentials của service account đang chạy.
package tasks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"

	"github.com/smartlife-lab/mimi/backend/internal/notify"
)

// Queue gửi task HTTP tới worker, có OIDC token của TasksSAEmail để worker (ingress internal,
// không cho truy cập công khai) chỉ nhận task từ hàng đợi.
type Queue struct {
	queue     string // projects/{p}/locations/{l}/queues/{q}
	workerURL string
	saEmail   string
	client    *http.Client
	endpoint  string
}

func New(ctx context.Context, queue, workerURL, saEmail string) (*Queue, error) {
	if queue == "" || workerURL == "" || saEmail == "" {
		return nil, fmt.Errorf("tasks: TASKS_QUEUE, WORKER_URL, TASKS_SA_EMAIL are required")
	}
	c, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("tasks: credentials: %w", err)
	}
	c.Timeout = 5 * time.Second
	return &Queue{queue: queue, workerURL: strings.TrimRight(workerURL, "/"), saEmail: saEmail, client: c, endpoint: "https://cloudtasks.googleapis.com/v2/"}, nil
}

var _ notify.Enqueuer = (*Queue)(nil)

// taskName suy từ uid và mã sự kiện: Cloud Tasks từ chối tên trùng (409) trong khoảng một giờ,
// nên đẩy lại cùng sự kiện không tạo thêm thư.
func (q *Queue) taskName(t notify.Task) string {
	h := sha256.Sum256([]byte(t.UID + "/" + t.EventID))
	return q.queue + "/tasks/ev-" + hex.EncodeToString(h[:16])
}

func (q *Queue) body(t notify.Task) ([]byte, error) {
	payload, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"task": map[string]any{
			"name": q.taskName(t),
			"httpRequest": map[string]any{
				"httpMethod": "POST",
				"url":        q.workerURL + "/tasks/notify",
				"headers":    map[string]string{"Content-Type": "application/json"},
				"body":       base64.StdEncoding.EncodeToString(payload),
				"oidcToken":  map[string]string{"serviceAccountEmail": q.saEmail, "audience": q.workerURL},
			},
		},
	})
}

func (q *Queue) Enqueue(ctx context.Context, t notify.Task) error {
	b, err := q.body(t)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.endpoint+q.queue+"/tasks", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("tasks: enqueue: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return nil // đã có task cùng tên
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tasks: enqueue: %s: %s", resp.Status, msg)
	}
	return nil
}
