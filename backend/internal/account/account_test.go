package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/household"
)

type fake struct {
	homes   map[string]household.Household
	calls   []string
	failRem bool
}

func (f *fake) Household(_ context.Context, hid string) (household.Household, error) {
	h, ok := f.homes[hid]
	if !ok {
		return household.Household{}, apperr.ErrNotFound
	}
	return h, nil
}
func (f *fake) RemoveMember(_ context.Context, hid, uid string) error {
	if f.failRem {
		return errors.New("firestore down")
	}
	f.calls = append(f.calls, "remove "+hid+" "+uid)
	return nil
}
func (f *fake) EraseHousehold(_ context.Context, hid string) error {
	f.calls = append(f.calls, "erase-home "+hid)
	delete(f.homes, hid)
	return nil
}
func (f *fake) EraseUser(_ context.Context, uid string) error {
	f.calls = append(f.calls, "erase-user "+uid)
	return nil
}
func (f *fake) DeleteUser(_ context.Context, uid string) error {
	f.calls = append(f.calls, "auth "+uid)
	return nil
}

func home() map[string]household.Household {
	return map[string]household.Household{"h1": {ID: "h1", OwnerUID: "me", Members: []household.Member{{UID: "me"}, {UID: "con"}, {UID: "bo"}}}}
}

func TestDeleteOwnerDissolvesHousehold(t *testing.T) {
	f := &fake{homes: home()}
	if err := New(f, f, f).Delete(context.Background(), authz.Principal{UID: "me", HouseholdID: "h1"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"remove h1 con", "remove h1 bo", "remove h1 me", "erase-home h1", "erase-user me", "auth me"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("%v", f.calls)
	}
}

func TestDeleteMemberOnlyLeaves(t *testing.T) {
	f := &fake{homes: home()}
	if err := New(f, f, f).Delete(context.Background(), authz.Principal{UID: "con", HouseholdID: "h1"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"remove h1 con", "erase-user con", "auth con"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("%v", f.calls)
	}
}

func TestDeleteResumesAndStopsBeforeAuth(t *testing.T) {
	// Lỗi giữa chừng: chưa xóa tài khoản đăng nhập, để người dùng còn đăng nhập lại được.
	f := &fake{homes: home(), failRem: true}
	if err := New(f, f, f).Delete(context.Background(), authz.Principal{UID: "me", HouseholdID: "h1"}); err == nil {
		t.Fatal("want error")
	}
	for _, c := range f.calls {
		if c == "auth me" {
			t.Fatal("auth deleted before data")
		}
	}
	// Gọi lại khi nhà đã bị xóa ở lần trước: bỏ qua bước nhà, xóa tiếp.
	f = &fake{homes: map[string]household.Household{}}
	if err := New(f, f, f).Delete(context.Background(), authz.Principal{UID: "me", HouseholdID: "h1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"erase-user me", "auth me"}) {
		t.Fatalf("%v", f.calls)
	}
}
