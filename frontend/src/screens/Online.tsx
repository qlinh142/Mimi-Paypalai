import { t, uiLang } from "../i18n";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Camera, CreditCard, Package, PackageCheck, ShieldCheck, Truck } from "lucide-react";
import { ApiError } from "../api/client";
import {
  useAdvanceOrder, useBasket, useCaptureOrder, useCardToken, useCheckout, usePayCard, useOrder, useOrders, useReadDelivery, useRefundOrder,
  useTaxonomyLabels, useVerifyOrder,
} from "../api/hooks";
import type { Basket, CardStatus, DeliveryObservation, Order, OrderLine, RefundReason } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { compressReceiptImage } from "../lib/image";
import { loadPayPal, PAYPAL_CLIENT_ID } from "../lib/paypal";
import { FAKE_CLIENT_TOKEN, FAKE_NONCE, loadDropin, threeDSecureRejected, type DropinInstance } from "../lib/braintree";
import { formatDayMonth, formatVnd } from "../lib/text";
import { displayCurrency } from "../lib/money";
import { m15 } from "../i18n/m15";

const usdFmt = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });
/**
 * Số tiền trong giỏ online: người xem chọn USD thì quy đổi theo đúng tỷ giá của đơn
 * (cùng tỷ giá PayPal sẽ thu), để tổng trên màn khớp số tiền thanh toán.
 */
const basketMoney = (vnd: number, rate: number) => (displayCurrency() === "USD" && rate > 0 ? usdFmt.format(Math.round((vnd * 100) / rate) / 100) : formatVnd(vnd));
const unitLabel = (u: string) => m15.units[u] ?? u;

/** Lý do ASK / BLOCK theo ngôn ngữ app; mã chưa biết thì dùng câu máy chủ gửi. */
function reasonText(r: Basket["decision"]["reasons"][number], b: Basket): string {
  const fn = t.online.reason[r.code];
  if (!fn) return r.detail;
  const l = b.lines.find((x) => x.key === r.key);
  const name = l?.name ?? r.key ?? "";
  if (r.code === "price_guard" && l) {
    const max = Math.floor((l.snapshot_price_vnd * 110) / 100);
    return fn(name, basketMoney(l.unit_price_vnd, b.vnd_per_usd), basketMoney(max, b.vnd_per_usd));
  }
  if (r.code === "delivery_guard" && l?.need_by) return fn(name, formatDayMonth(l.eta), formatDayMonth(l.need_by));
  return fn(name, "", "");
}

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
  const label = useTaxonomyLabels();
  const rate = basket.vnd_per_usd;
  const usd = displayCurrency() === "USD";
  return (
    <>
      <section className="plan-sec">
        <h2>{t.online.merchant(basket.merchant)}</h2>
        <p className="hint">{t.online.snapshot(formatDayMonth(basket.snapshot_at.slice(0, 10)))}</p>
        <ul className="list">
          {basket.lines.map((l) => (
            <li key={l.key} className="plan-row plan-row--online">
              <span className="plan-row__text">
                <span className="plan-row__name">{l.product_name}</span>
                <span className="plan-row__sub">
                  {t.online.packs(l.qty, unitLabel(l.unit), l.pack_qty)} · {t.online.eta(formatDayMonth(l.eta))}
                </span>
                <span className="plan-row__sub">
                  {t.online.priceVsSnapshot(basketMoney(l.unit_price_vnd, rate), basketMoney(l.snapshot_price_vnd, rate))}
                </span>
              </span>
              <span className="num">{basketMoney(l.total_vnd, rate)}</span>
            </li>
          ))}
        </ul>
        {basket.missing.length > 0 && <p className="hint">{t.online.missing(basket.missing.map((k) => label(k) || k).join(", "))}</p>}
        <p className="plan-total">
          <span>{t.online.total}</span>
          <span className="num">{usd ? usdFmt.format(Number(basket.amount_usd)) : formatVnd(basket.total_vnd)}</span>
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
                {reasonText(r, basket)}
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
      {canPay && basket.card_enabled && <CardButton approve={decision.result === "ASK"} />}
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

/**
 * Trả bằng thẻ qua Braintree, nằm ngay dưới nút PayPal. Bấm lần đầu: lấy client token và
 * mở Drop-in (ô nhập thẻ do Braintree dựng, số thẻ không qua server Mimi). Bấm lần hai:
 * Drop-in trả nonce, app tạo đơn provider=braintree rồi gửi nonce để server thu tiền.
 * Server chạy BRAINTREE_MODE=fake (hoặc bản thử) thì thu luôn bằng nonce thử.
 */
function CardButton({ approve }: { approve: boolean }) {
  const token = useCardToken();
  const checkout = useCheckout();
  const pay = usePayCard();
  const navigate = useNavigate();
  const box = useRef<HTMLDivElement>(null);
  const dropin = useRef<DropinInstance | null>(null);
  // Đơn đã tạo nhưng thẻ bị từ chối: thử lại trên cùng đơn, không tạo đơn chờ mới.
  const orderId = useRef("");
  const amountUSD = useRef("");
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const busy = token.isPending || checkout.isPending || pay.isPending;

  useEffect(() => () => void dropin.current?.teardown().catch(() => undefined), []);

  // Đơn tạo trước khi hỏi thẻ để 3D Secure xác minh đúng số tiền của đơn.
  const ensureOrder = async () => {
    if (!orderId.current) {
      const o = await checkout.mutateAsync({ keys: [], approve, provider: "braintree" });
      orderId.current = o.id;
      amountUSD.current = o.amount_usd;
    }
  };

  const charge = async (nonce: string) => {
    const paid = await pay.mutateAsync({ id: orderId.current, nonce });
    navigate(`/orders/${paid.id}`);
  };

  const onClick = async () => {
    if (busy) return;
    setError("");
    try {
      if (dropin.current) {
        await ensureOrder();
        const p = await dropin.current.requestPaymentMethod({ threeDSecure: { amount: amountUSD.current } });
        if (threeDSecureRejected(p)) {
          setError(t.online.card3dsFailed);
          return;
        }
        await charge(p.nonce);
        return;
      }
      const auth = await token.mutateAsync();
      if (auth === FAKE_CLIENT_TOKEN) {
        await ensureOrder();
        await charge(FAKE_NONCE);
        return;
      }
      setOpen(true);
      const dropinNs = await loadDropin();
      if (!box.current) return;
      dropin.current = await dropinNs.create({
        authorization: auth,
        container: box.current,
        locale: uiLang === "vi" ? "vi_VN" : "en_US",
        threeDSecure: true,
      });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t.online.failed);
    }
  };

  return (
    <section className="plan-sec">
      <p className="hint">{t.online.cardOr}</p>
      <div ref={box} hidden={!open} />
      <button type="button" className="btn btn--quiet btn--block" aria-disabled={busy || undefined} onClick={onClick}>
        <CreditCard size={18} aria-hidden /> {busy ? t.online.paying : open ? t.online.cardConfirm : t.online.payCard}
      </button>
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
        {orders.data && orders.data.length === 0 && (
          // B3 (theo Redesign): trang trống có icon, tiêu đề và dòng phụ.
          <div className="empty22" role="status">
            <span className="empty22__ill" aria-hidden><Package size={30} /></span>
            <strong>{m15.settings.ordersEmptyTitle}</strong>
            <span className="hint">{m15.settings.ordersEmptySub}</span>
          </div>
        )}
        <ul className="list">
          {(orders.data ?? []).map((o) => (
            <li key={o.id}>
              <Link to={`/orders/${o.id}`} className="plan-row">
                <Truck size={20} aria-hidden />
                <span className="plan-row__text">
                  <span className="plan-row__name">{o.merchant} · {formatDayMonth(o.created_at.slice(0, 10))}</span>
                  <span className="plan-row__sub">{t.orders.states[o.state]}</span>
                </span>
                <span className="num">{basketMoney(o.state === "awaiting_payment" ? o.total_vnd : o.net_vnd, o.vnd_per_usd)}</span>
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

/** Tin tiền thẻ cần người dùng chú ý: tiền chưa về hoặc bị khiếu nại. */
const CARD_ALERT = new Set<CardStatus>(["settlement_declined", "disputed", "dispute_lost"]);

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
              {t.orders.states[e.state]} · {new Date(e.at).toLocaleString(uiLang === "vi" ? "vi-VN" : "en-US", { dateStyle: "short", timeStyle: "short" })}
            </li>
          ))}
        </ol>
        <p className="plan-total">
          <span>{t.orders.total(displayCurrency() === "USD" ? usdFmt.format(Number(order.amount_usd)) : formatVnd(order.total_vnd))}</span>
          <span className="num">US${order.amount_usd}</span>
        </p>
        {order.state !== "awaiting_payment" && <p className="hint">{t.orders.paidVia[order.provider ?? "paypal"]}</p>}
        {order.card_status && (
          <p className={CARD_ALERT.has(order.card_status) ? "error" : "hint"} role={CARD_ALERT.has(order.card_status) ? "alert" : undefined}>
            {t.orders.cardStatus[order.card_status]}
          </p>
        )}
        {order.refunded_vnd > 0 && <p className="hint">{t.orders.net(basketMoney(order.net_vnd, order.vnd_per_usd))}</p>}
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

      {order.state === "delivered" ? <VerifyForm order={order} /> : <LinesView lines={order.lines} rate={order.vnd_per_usd} />}

      {order.state === "verified" && (
        <>
          <p className="hint">{t.orders.reconciled}</p>
          {order.refunds.map((r) => (
            <p key={r.ref} className="check ok">{t.orders.refunded(basketMoney(r.amount_vnd, order.vnd_per_usd), r.amount_usd)}</p>
          ))}
          {order.suggested_refund_vnd > 0 && <RefundForm order={order} />}
          {/* Đơn đã kiểm xong: một nút để rời màn đơn, món thiếu vẫn nằm trong Kế hoạch. */}
          <Link to="/plan" className={`btn btn--block ${order.suggested_refund_vnd > 0 ? "btn--quiet" : "btn--primary"}`}>
            {t.orders.done}
          </Link>
        </>
      )}
    </>
  );
}

function LinesView({ lines, rate }: { lines: OrderLine[]; rate: number }) {
  return (
    <section className="plan-sec">
      <ul className="list">
        {lines.map((l) => (
          <li key={l.key} className="plan-row plan-row--online">
            <span className="plan-row__text">
              <span className="plan-row__name">{l.product_name}</span>
              <span className="plan-row__sub">
                {t.orders.expected} {l.qty}
                {l.verified_qty != null && ` · ${t.orders.received} ${l.verified_qty}`}
                {l.match && ` · ${t.orders.match[l.match]}`}
              </span>
            </span>
            <span className="num">{basketMoney(l.total_vnd, rate)}</span>
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

/** Refund (Mimi 2.0 mục 18): hoàn qua cổng đã thu tiền (PayPal hoặc thẻ Braintree), giữ giao dịch gốc để audit. */
function RefundForm({ order }: { order: Order }) {
  const refund = useRefundOrder();
  const [reason, setReason] = useState<RefundReason>("missing");
  const [note, setNote] = useState("");
  const amount = order.suggested_refund_vnd;
  return (
    <section className="plan-sec">
      <h2>{t.orders.refundTitle}</h2>
      <p className="hint">{t.orders.refundSuggested(basketMoney(amount, order.vnd_per_usd))}</p>
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
          {t.orders.refund(basketMoney(amount, order.vnd_per_usd))}
        </button>
      </form>
    </section>
  );
}
