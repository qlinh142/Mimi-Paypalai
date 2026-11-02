package fsstore

import "testing"

func TestMigrationsOrdered(t *testing.T) {
	for i, m := range migrations {
		if m.Version != i+1 || m.Name == "" {
			t.Fatalf("bước %d: Version phải là %d và có tên, got %+v", i, i+1, m)
		}
	}
	if SchemaVersion != len(migrations) {
		t.Fatalf("SchemaVersion=%d", SchemaVersion)
	}
}

func TestPendingMigrations(t *testing.T) {
	if got := pendingMigrations(0); len(got) != len(migrations) {
		t.Fatalf("tài liệu cũ chạy mọi bước, got %d", len(got))
	}
	if got := pendingMigrations(SchemaVersion); len(got) != 0 {
		t.Fatalf("đã ở bản mới thì không chạy gì, got %d", len(got))
	}
	if got := pendingMigrations(SchemaVersion + 5); len(got) != 0 {
		t.Fatalf("dữ liệu mới hơn code (sau khi quay lại bản cũ) thì để nguyên, got %d", len(got))
	}
}
