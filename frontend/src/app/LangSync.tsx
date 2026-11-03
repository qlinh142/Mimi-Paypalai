import { useEffect, useRef } from "react";
import { useMe, useSaveSettings } from "../api/hooks";
import { switchUiLang, uiLang } from "../i18n";

/**
 * Giữ ngôn ngữ trên máy khớp với tài khoản (mẹ chốt 03/11: ngôn ngữ theo tài khoản).
 * Tài khoản chưa có ngôn ngữ (người mới): ghi ngôn ngữ đang dùng trên máy lên tài khoản.
 * Tài khoản đã có ngôn ngữ khác máy: đổi app theo tài khoản, trừ khi vừa mở bằng ?ui=.
 */
export function LangSync() {
  const me = useMe();
  const save = useSaveSettings();
  const done = useRef(false);
  // Chỉ tin /me vừa tải từ máy chủ sau khi mở app: bản khôi phục từ IndexedDB có thể còn ngôn
  // ngữ cũ ngay sau khi đổi trong Cài đặt rồi tải lại trang.
  const openedAt = useRef(Date.now());
  const fresh = !!me.data && me.dataUpdatedAt >= openedAt.current;
  const account = me.data?.settings.lang;
  useEffect(() => {
    if (!fresh || done.current) return;
    done.current = true;
    const fromUrl = new URLSearchParams(location.search).get("ui");
    if (!account || fromUrl) {
      if (account !== uiLang) save.mutate({ lang: uiLang });
      return;
    }
    if (account !== uiLang) switchUiLang(account);
  }, [fresh, account, save]);
  return null;
}
