package authz

import (
	"errors"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

func TestCan(t *testing.T) {
	owner := Principal{UID: "u1", Role: RoleOwner}
	member := Principal{UID: "u2", Role: RoleMember, HouseholdID: "h1"}
	tests := []struct {
		name string
		p    Principal
		a    Action
		want apperr.Code // "" = allowed
	}{
		{"owner reads spending", owner, ActionReadSpending, ""},
		{"member reads own spending", member, ActionReadSpending, ""},
		{"member manages own items", member, ActionWriteItems, ""},
		{"member cannot manage household", member, ActionManageHousehold, apperr.CodeForbidden},
		{"member assigns shopping", member, ActionAssignShopping, ""},
		{"solo cannot assign shopping", owner, ActionAssignShopping, apperr.CodeForbidden},
		{"member reads shared items", member, ActionReadItemsShared, ""},
		{"member answers ask", member, ActionAnswerAsk, ""},
		{"owner cannot answer own ask", owner, ActionAnswerAsk, apperr.CodeForbidden},
		{"zero role gets nothing", Principal{UID: "u3"}, ActionReadSpending, apperr.CodeForbidden},
		{"no uid is unauthenticated", Principal{}, ActionReadSelf, apperr.CodeUnauthenticated},
		{"unknown action denied", owner, ActionUnknown, apperr.CodeForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Can(tt.p, tt.a)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("want allowed, got %v", err)
				}
				return
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Code != tt.want {
				t.Fatalf("want %s, got %v", tt.want, err)
			}
		})
	}
}
