import { t } from "../i18n";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Camera, PackageCheck, ShieldCheck, Truck } from "lucide-react";
import { ApiError } from "../api/client";
import {
  useAdvanceOrder, useBasket, useCaptureOrder, useCheckout, useOrder, useOrders, useReadDelivery, useRefundOrder,
  useVerifyOrder,
} from "../api/hooks";
import type { Basket, DeliveryObservation, Order, OrderLine, RefundReason } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { compressReceiptImage } from "../lib/image";
import { loadPayPal, PAYPAL_CLIENT_ID } from "../lib/paypal";
import { formatDayMonth, formatVnd } from "../lib/text";

/**
 * Mua online (Mimi 2.0 mục 12 đến 15): giỏ online dựng từ Kế hoạch, Household Mandate,
 * thanh toán PayPal. Mọi con số và kết quả ALLOW / ASK / BLOCK do API tính.
 */
export function OnlineBuy() {
  const basket = useBasket([]);
  const b = basket.data;
  return (
    <MainFrame>
      <TopBar title={t.online.title} variant="sub" />
      <div className="page plan">
        {basket.isPending && <p className="hint">{t.common.loading}</p>}
        {basket.isError && (
          <div className="empty">
            <p>{basket.error instanceof ApiError ? basket.error.message : t.common.genericError}</p>
            <button className="btn btn--quiet" onClick={() => basket.refetch()}>{t.common.retry}</button>
          </div>
        )}
        {b && b.lines.length === 0 && <p className="hint">{t.online.empty}</p>}
        {b && b.lines.length > 0 && <BasketView basket={b} />}
      </div>
    </MainFrame>
  );
}

function BasketView({ basket }: { basket: Basket }) {
  const [approve, setApprove] = useState(false);
  const decision = basket.decision;
  const canPay = decision.result === "ALLOW" || (decision.result === "ASK" && approve);
  return (
    <>
      <section className="plan-sec">
        <h2>{t.online.merchant(basket.merchant)}</h2>
        <p className="hint">{t.online.snapshot(formatDayMonth(basket.snapshot_at.slice(0, 10)))}</p>
        <ul className="list">
          {basket.lines.map((l) => (
            <li key={l.key} className="plan-row">
              <span className="plan-row__text">
                <span className="plan-row__name">{l.product_name}</span>
                <span className="plan-row__sub">
                  {t.online.packs(l.qty, l.unit, l.pack_qty)} · {t.online.eta(formatDayMonth(l.eta))}
                </span>
                <span className="plan-row__sub">
                  {t.online.priceVsSnapshot(formatVnd(l.unit_price_vnd), formatVnd(l.snapshot_price_vnd))}
                </span>
              </span>
              <span className="num">{formatVnd(l.total_vnd)}</span>
            </li>
          ))}
        </ul>
        {basket.missing.length > 0 && <p className="hint">{t.online.missing(basket.missing.join(", "))}</p>}
        <p className="plan-total">
          <span>{t.online.total}</span>
          <span className="num">{formatVnd(basket.total_vnd)}</span>
        </p>
        <p className="hint">{t.online.usd(basket.amount_usd, basket.vnd_per_usd)}</p>
      </section>

      <section className="plan-sec" aria-live="polite">
        <h2>
          <ShieldCheck size={18} aria-hidden /> {t.online.results[decision.result]}
        </h2>
        {decision.reasons.length > 0 && (
          <ul className="list">
            {decision.reasons.map((r, i) => (
              <li key={`${r.code}-${r.key ?? i}`} className={`check ${r.result === "BLOCK" ? "warn" : ""}`}>
                {r.detail}
              </li>
            ))}
          </ul>
        )}
        {decision.result === "BLOCK" && (
          <p className="hint">
            {t.online.blocked} <Link to="/settings">{t.settings.title}</Link>
          </p>
        )}
        {decision.result === "ASK" && (
          <label className="check">
            <input type="checkbox" checked={approve} onChange={(e) => setApprove(e.target.checked)} /> {t.online.approve}
          </label>
        )}
      </section>

      {canPay && <PayButton approve={decision.result === "ASK"} />}
    </>
  );
}

/**
 * Có VITE_PAYPAL_CLIENT_ID: nút PayPal thật (Sandbox). createOrder gọi POST /orders để
 * backend áp Mandate rồi tạo PayPal order; onApprove gọi capture. Không có: nút giả lập,
 * backend chạy PAYPAL_MODE=fake và capture ngay.
 */
function PayButton({ approve }: { approve: boolean }) {
  const checkout = useCheckout();
  const capture = useCaptureOrder();
  const navigate = useNavigate();
  const box = useRef<HTMLDivElement>(null);
  const orderId = useRef("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!PAYPAL_CLIENT_ID || !box.current) return;
    let cancelled = false;
    const el = box.current;
    loadPayPal()
      .then((pp) => {
        if (cancelled) return;
        el.innerHTML = "";
        return pp
          .Buttons({
            style: { layout: "vertical", shape: "rect", label: "pay" },
            createOrder: async () => {
              setError("");
              const o = await checkout.mutateAsync({ keys: [], approve });
              orderId.current = o.id;
              return o.paypal_order_id;
            },
            onApprove: async () => {
              const o = await capture.mutateAsync(orderId.current);
              navigate(`/orders/${o.id}`);
            },
            onError: () => setError(t.online.failed),
          })
          .render(el);
      })
      .catch(() => setError(t.online.failed));
    return () => {
      cancelled = true;
    };
    // Nút PayPal dựng một lần cho mỗi lựa chọn duyệt.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [approve]);

  const busy = checkout.isPending || capture.isPending;
  const simulate = async () => {
    if (busy) return;
    setError("");
    try {
      const o = await checkout.mutateAsync({ keys: [], approve });
      if (!o.paypal_order_id.startsWith("FAKE-")) {
        // Backend nối PayPal thật nhưng app thiếu client ID: mở trang approve của PayPal.
        if (o.approve_url) window.location.assign(o.approve_url);
        return;
      }
      const paid = await capture.mutateAsync(o.id);
      navigate(`/orders/${paid.id}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t.online.failed);
    }
  };

  return (
    <section className="plan-sec">
      {PAYPAL_CLIENT_ID ? (
        <div ref={box} aria-label={t.online.pay} />
      ) : (
        <button type="button" className="btn btn--primary btn--block" aria-disabled={busy || undefined} onClick={simulate}>
          {busy ? t.online.paying : t.online.paySandbox}
        </button>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </section>
  );
}

// ================= Đơn online: danh sách và chi tiết =================

export function Orders() {
  const orders = useOrders();
  return (
    <MainFrame>
      <TopBar title={t.orders.title} variant="sub" />
      <div className="page plan">
        {orders.isPending && <p className="hint">{t.common.loading}</p>}
        {orders.isError && <p className="error">{t.common.genericError}</p>}
        {orders.data && orders.data.length === 0 && <p className="hint">{t.orders.empty}</p>}
        <ul className="list">
          {(orders.data ?? []).map((o) => (
            <li key={o.id}>
              <Link to={`/orders/${o.id}`} className="plan-row">
                <Truck size={20} aria-hidden />
                <span className="plan-row__text">
                  <span className="plan-row__name">{o.merchant} · {formatDayMonth(o.created_at.slice(0, 10))}</span>
                  <span className="plan-row__sub">{t.orders.states[o.state]}</span>
                </span>
                <span className="num">{formatVnd(o.state === "awaiting_payment" ? o.total_vnd : o.net_vnd)}</span>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </MainFrame>
  );
}

export function OrderDetail() {
  const { id = "" } = useParams();
  const q = useOrder(id);
  const o = q.data;
  return (
    <MainFrame>
      <TopBar title={t.orders.title} variant="sub" />
      <div className="page plan">
        {q.isPending && <p className="hint">{t.common.loading}</p>}
        {q.isError && <p className="error">{q.error instanceof ApiError ? q.error.message : t.common.genericError}</p>}
        {o && <OrderView order={o} />}
      </div>
    </MainFrame>
  );
}

function OrderView({ order }: { order: Order }) {
  const advance = useAdvanceOrder();
  const canAdvance = order.state === "paid" || order.state === "ordered" || order.state === "shipped";
  return (
    <>
      <section className="plan-sec">
        <h2>{t.orders.states[order.state]}</h2>
        <ol className="list">
          {order.events.map((e) => (
            <li key={`${e.state}-${e.at}`} className="plan-row__sub">
              {t.orders.states[e.state]} · {new Date(e.at).toLocaleString()}
            </li>
          ))}
        </ol>
        <p className="plan-total">
          <span>{t.orders.total(formatVnd(order.total_vnd))}</span>
          <span className="num">US${order.amount_usd}</span>
        </p>
        {order.refunded_vnd > 0 && <p className="hint">{t.orders.net(formatVnd(order.net_vnd))}</p>}
        {canAdvance && (
          <>
            <button
              type="button"
              className="btn btn--quiet btn--block"
              aria-disabled={advance.isPending || undefined}
              onClick={() => !advance.isPending && advance.mutate(order.id)}
            >
              <Truck size={18} aria-hidden /> {t.orders.advance}
            </button>
            <p className="hint">{t.orders.sandboxNote}</p>
          </>
        )}
      </section>

      {order.state === "delivered" ? <VerifyForm order={order} /> : <LinesView lines={order.lines} />}

      {order.state === "verified" && (
        <>
          <p className="hint">{t.orders.reconciled}</p>
          {order.refunds.map((r) => (
            <p key={r.ref} className="check ok">{t.orders.refunded(formatVnd(r.amount_vnd), r.amount_usd)}</p>
          ))}
          {order.suggested_refund_vnd > 0 && <RefundForm order={order} />}
        </>
      )}
    </>
  );
}

function LinesView({ lines }: { lines: OrderLine[] }) {
  return (
    <section className="plan-sec">
      <ul className="list">
        {lines.map((l) => (
          <li key={l.key} className="plan-row">
            <span className="plan-row__text">
              <span className="plan-row__name">{l.product_name}</span>
              <span className="plan-row__sub">
                {t.orders.expected} {l.qty}
                {l.verified_qty != null && ` · ${t.orders.received} ${l.verified_qty}`}
                {l.match && ` · ${t.orders.match[l.match]}`}
              </span>
            </span>
            <span className="num">{formatVnd(l.total_vnd)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * Verify (Mimi 2.0 mục 17): AI đếm từ ảnh chỉ là gợi ý; người dùng xác nhận con số.
 * Delivered không phải Verified: chỉ số đã xác nhận mới vào tồn kho.
 */
function VerifyForm({ order }: { order: Order }) {
  const read = useReadDelivery();
  const verify = useVerifyOrder();
  const [actual, setActual] = useState<Record<string, string>>(
    Object.fromEntries(order.lines.map((l) => [l.key, String(l.qty)])),
  );
  const [obs, setObs] = useState<Record<string, DeliveryObservation>>({});
  const file = useRef<HTMLInputElement>(null);

  const onPhotos = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const images = await Promise.all([...files].slice(0, 4).map((f) => compressReceiptImage(f)));
    read.mutate(
      { id: order.id, images },
      {
        onSuccess: (items) => {
          setObs(Object.fromEntries(items.map((i) => [i.key, i])));
          setActual((cur) => ({ ...cur, ...Object.fromEntries(items.map((i) => [i.key, String(i.qty)])) }));
        },
      },
    );
  };

  const values = Object.fromEntries(Object.entries(actual).map(([k, v]) => [k, Number(v.replace(",", "."))]));
  const valid = Object.values(values).every((v) => Number.isFinite(v) && v >= 0);

  return (
    <section className="plan-sec">
      <h2>
        <PackageCheck size={18} aria-hidden /> {t.orders.verifyTitle}
      </h2>
      <p className="hint">{t.orders.verifyHint}</p>
      <input
        ref={file}
        type="file"
        accept="image/*"
        capture="environment"
        multiple
        hidden
        onChange={(e) => onPhotos(e.target.files)}
      />
      <button type="button" className="btn btn--quiet btn--block" aria-disabled={read.isPending || undefined} onClick={() => file.current?.click()}>
        <Camera size={18} aria-hidden /> {read.isPending ? t.orders.reading : t.orders.readPhoto}
      </button>
      {read.isError && <p className="error" role="alert">{read.error instanceof ApiError ? read.error.message : t.common.genericError}</p>}
      <form
        className="plan-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!valid || verify.isPending) return;
          verify.mutate({ id: order.id, actual: values });
        }}
      >
        {order.lines.map((l) => (
          <label key={l.key}>
            {l.product_name} · {t.orders.expected} {l.qty}
            {obs[l.key] && ` · ${t.orders.confidence[obs[l.key].confidence]}`}
            <input
              inputMode="decimal"
              value={actual[l.key] ?? ""}
              aria-label={`${l.product_name} ${t.orders.received}`}
              onChange={(e) => setActual({ ...actual, [l.key]: e.target.value })}
            />
          </label>
        ))}
        {verify.isError && <p className="error" role="alert">{verify.error instanceof ApiError ? verify.error.message : t.common.genericError}</p>}
        <button type="submit" className="btn btn--primary btn--block" aria-disabled={!valid || verify.isPending || undefined}>
          {t.orders.confirm}
        </button>
      </form>
    </section>
  );
}

const REASONS: RefundReason[] = ["missing", "wrong", "damaged", "other"];

/** Refund (Mimi 2.0 mục 18): hoàn qua PayPal Sandbox, giữ giao dịch gốc để audit. */
function RefundForm({ order }: { order: Order }) {
  const refund = useRefundOrder();
  const [reason, setReason] = useState<RefundReason>("missing");
  const [note, setNote] = useState("");
  const amount = order.suggested_refund_vnd;
  return (
    <section className="plan-sec">
      <h2>{t.orders.refundTitle}</h2>
      <p className="hint">{t.orders.refundSuggested(formatVnd(amount))}</p>
      <form
        className="plan-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (refund.isPending) return;
          refund.mutate({ id: order.id, reason, amount_vnd: amount, note: note.trim() });
        }}
      >
        <label>
          {t.orders.reason}
          <select value={reason} onChange={(e) => setReason(e.target.value as RefundReason)}>
            {REASONS.map((r) => (
              <option key={r} value={r}>{t.orders.reasons[r]}</option>
            ))}
          </select>
        </label>
        <label>
          {t.orders.note}
          <textarea maxLength={200} value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        {refund.isError && <p className="error" role="alert">{refund.error instanceof ApiError ? refund.error.message : t.common.genericError}</p>}
        <button type="submit" className="btn btn--primary btn--block" aria-disabled={refund.isPending || undefined}>
          {t.orders.refund(formatVnd(amount))}
        </button>
      </form>
    </section>
  );
}
