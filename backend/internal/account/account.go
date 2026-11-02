// Package account xóa tài khoản và toàn bộ dữ liệu của một người (B10, DELETE /me).
package account

import (
	"context"
	"fmt"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/household"
)

// Households là phần nhà cần để rời hoặc giải tán nhà.
type Households interface {
	Household(ctx context.Context, hid string) (household.Household, error)
	// RemoveMember bỏ người khỏi nhà, hủy vòng mua hộ còn mở có người đó.
	RemoveMember(ctx context.Context, hid, uid string) error
}

// Eraser xóa đệ quy dữ liệu trên Firestore. Gọi lại khi đã xóa thì không lỗi.
type Eraser interface {
	EraseHousehold(ctx context.Context, hid string) error
	EraseUser(ctx context.Context, uid string) error
}

// AuthUsers xóa tài khoản đăng nhập. Không có tài khoản thì coi như đã xóa.
type AuthUsers interface {
	DeleteUser(ctx context.Context, uid string) error
}

type Service struct {
	homes Households
	data  Eraser
	auth  AuthUsers
}

func New(homes Households, data Eraser, auth AuthUsers) *Service {
	return &Service{homes: homes, data: data, auth: auth}
}

// Delete xóa theo thứ tự để gọi lại sau lỗi giữa chừng vẫn chạy tiếp được:
//  1. Nhà: thành viên thì rời nhà; chủ nhà thì mời từng người ra (họ về nhà riêng, vòng
//     mua hộ còn mở bị hủy, món trả về kế hoạch của người nhờ), rồi xóa nhà.
//  2. Xóa đệ quy users/{uid}: kế hoạch, hóa đơn, lịch sử mua, thông báo, cài đặt.
//  3. Xóa tài khoản đăng nhập sau cùng, để lỗi ở bước trước vẫn đăng nhập lại và xóa tiếp được.
//
// Dữ liệu đã nằm trong sổ của người khác (món mình mua hộ người nhà) là của họ, không xóa.
func (s *Service) Delete(ctx context.Context, p authz.Principal) error {
	if p.UID == "" {
		return apperr.ErrUnauthenticated
	}
	if p.HouseholdID != "" {
		if err := s.leaveHousehold(ctx, p); err != nil {
			return err
		}
	}
	if err := s.data.EraseUser(ctx, p.UID); err != nil {
		return fmt.Errorf("account erase user: %w", err)
	}
	if err := s.auth.DeleteUser(ctx, p.UID); err != nil {
		return fmt.Errorf("account delete auth: %w", err)
	}
	return nil
}

func (s *Service) leaveHousehold(ctx context.Context, p authz.Principal) error {
	h, err := s.homes.Household(ctx, p.HouseholdID)
	if apperr.From(err).Code == apperr.CodeNotFound {
		return nil // đã xóa ở lần gọi trước
	}
	if err != nil {
		return err
	}
	if h.OwnerUID != p.UID {
		return s.homes.RemoveMember(ctx, h.ID, p.UID)
	}
	for _, m := range h.Members {
		if m.UID == p.UID {
			continue
		}
		if err := s.homes.RemoveMember(ctx, h.ID, m.UID); err != nil {
			return fmt.Errorf("account remove member: %w", err)
		}
	}
	if err := s.homes.RemoveMember(ctx, h.ID, p.UID); err != nil {
		return fmt.Errorf("account remove owner: %w", err)
	}
	return s.data.EraseHousehold(ctx, h.ID)
}
