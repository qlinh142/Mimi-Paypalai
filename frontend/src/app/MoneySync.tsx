import { useEffect, useRef, useState, type ReactNode } from "react";
import { apiGet } from "../api/client";
import { useMe } from "../api/hooks";
import { displayCurrency, hasRates, setRates, switchCurrency } from "../lib/money";

type RatesResp = { currency: string; rates: { on: string; rate: string }[] };

/**
 * Giữ tiền tệ hiển thị khớp với tài khoản và nạp tỷ giá USD từng ngày (mẹ chốt 03/11).
 * Đang xem USD mà máy chưa có tỷ giá thì chờ nạp xong mới hiện màn, để không lóe số VND.
 * Không nạp được (mất mạng, máy chủ lỗi) thì vẫn hiện, số tiền tạm để VND.
 */
export function MoneySync({ children }: { children: ReactNode }) {
  const me = useMe();
  const [ready, setReady] = useState(() => displayCurrency() === "VND" || hasRates());
  // Chỉ tin bản vừa tải từ máy chủ sau khi mở app: bản nhớ trên máy (khôi phục từ IndexedDB)
  // có thể còn tiền tệ cũ ngay sau khi đổi rồi tải lại trang.
  const openedAt = useRef(Date.now());
  const fresh = !!me.data && me.dataUpdatedAt >= openedAt.current;
  const account = fresh ? (me.data?.settings.currency ?? "VND") : undefined;

  useEffect(() => {
    if (account && account !== displayCurrency()) switchCurrency(account);
  }, [account]);

  useEffect(() => {
    if (displayCurrency() !== "USD") return;
    let alive = true;
    apiGet<RatesResp>("/fx/rates?currency=USD")
      .then((r) => {
        const pts = r.rates.map((p) => ({ on: p.on, rate: Number(p.rate) })).filter((p) => p.rate > 0);
        if (pts.length) setRates(pts);
      })
      .catch(() => undefined)
      .finally(() => alive && setReady(true));
    return () => {
      alive = false;
    };
  }, []);

  return ready ? <>{children}</> : null;
}
