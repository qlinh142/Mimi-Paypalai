package tasks

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/notify"
)

func TestTaskBody(t *testing.T) {
	q := &Queue{queue: "projects/p/locations/asia-southeast1/queues/notify", workerURL: "https://worker", saEmail: "sa-tasks@p.iam.gserviceaccount.com"}
	task := notify.Task{UID: "minh", EventID: "sent-trip-1"}
	b, err := q.body(task)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Task struct {
			Name        string `json:"name"`
			HTTPRequest struct {
				URL       string            `json:"url"`
				Body      string            `json:"body"`
				OIDCToken map[string]string `json:"oidcToken"`
			} `json:"httpRequest"`
		} `json:"task"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(v.Task.HTTPRequest.Body)
	var got notify.Task
	_ = json.Unmarshal(raw, &got)
	if got != task || v.Task.HTTPRequest.URL != "https://worker/tasks/notify" || v.Task.HTTPRequest.OIDCToken["serviceAccountEmail"] == "" {
		t.Fatalf("%s", b)
	}
	// Cùng sự kiện thì cùng tên task (Cloud Tasks bỏ task trùng), khác sự kiện thì khác tên.
	if q.taskName(task) != v.Task.Name || q.taskName(task) == q.taskName(notify.Task{UID: "minh", EventID: "upd-trip-1-1"}) {
		t.Fatal("task name must follow the event")
	}
}
