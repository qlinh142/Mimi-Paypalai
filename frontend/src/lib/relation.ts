import { useMe, useSaveSettings } from "../api/hooks";

/**
 * Tên gọi người nhà (Anh, Chị, Bố, Mẹ...) mà mỗi tài khoản tự đặt cho người khác. Lưu trên máy
 * chủ trong settings.relations, nên đăng nhập lại vẫn còn. Danh sách khớp user.Relations ở backend.
 */
export const RELATIONS = ["anh", "chi", "em", "bo", "me", "con", "ong", "ba", "khac"] as const;

export function useRelations() {
  const me = useMe();
  const save = useSaveSettings();
  // Đang lưu thì hiện ngay giá trị vừa chọn, không chờ /me tải lại.
  const map = { ...(me.data?.settings.relations ?? {}), ...(save.isPending ? save.variables?.relations : undefined) };
  return {
    relationOf: (uid: string): string => map[uid] || "",
    setRelation: (uid: string, rel: string) => save.mutate({ relations: { [uid]: rel } }),
    saving: save.isPending,
    error: save.isError,
  };
}
