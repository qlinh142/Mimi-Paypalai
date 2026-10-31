/**
 * Vai trong nhà của từng người (Anh, Chị, Bố, Mẹ...). API chưa có trường này nên tạm lưu trên máy;
 * khi backend thêm trường thì chuyển sang lưu cùng thành viên.
 */
export const RELATIONS = ["anh", "chi", "em", "bo", "me", "con", "ong", "ba", "khac"] as const;
const KEY = (uid: string) => `mimi.relation.${uid}`;

export function relationOf(uid: string): string {
  try {
    return localStorage.getItem(KEY(uid)) ?? "";
  } catch {
    return "";
  }
}

export function setRelation(uid: string, rel: string) {
  try {
    if (rel) localStorage.setItem(KEY(uid), rel);
    else localStorage.removeItem(KEY(uid));
  } catch {
    // bỏ qua
  }
}
