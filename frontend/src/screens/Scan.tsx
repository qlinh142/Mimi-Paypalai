import { t, uiLang } from "../i18n";
import { m15 } from "../i18n/m15";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Camera, ChevronRight, ImagePlus, X } from "lucide-react";
import { toSaveRequest, useDelegated, usePlanCarts, usePlans, useReadReceipt, useSaveReceipt } from "../api/hooks";
import { allocate, planNeeds } from "../lib/allocate";
import { visiblePlans } from "../components/PlanPicker";
import { planName } from "./PlanList";
import { m1 } from "../i18n/m1";
import { currentPlanId } from "../lib/currentPlan";
import type { ReceiptDraft } from "../api/types";
import { ApiError, OfflineError, apiUploadReceipt } from "../api/client";
import { compressReceiptImage, grabFrame } from "../lib/image";
import { formatDayMonth, formatVnd } from "../lib/text";
import { MimiMark } from "../components/MimiMark";
import { type EditLine, ReceiptLines, parseAmount } from "../components/ReceiptLines";

const MAX_PHOTOS = 10; // BUG-05 mục 6: tối đa 10 ảnh / hóa đơn, khớp giới hạn backend

type Photo = { id: string; blob: Blob; url: string; from: "camera" | "library" };

const MAX_BILLS = 2; // mẹ chốt 02/11: tối đa 2 bill mỗi lần quét, mỗi bill đọc riêng

/** Bill 1 được đọc ngầm trong lúc người dùng chụp bill 2. */
export type BgBill = {
  receiptId: string;
  images: Blob[];
  status: "reading" | "ok" | "error";
  draft?: ReceiptDraft;
  error?: unknown;
};

/**
 * Mỗi lần "Quét hóa đơn khác" tạo phiên mới: ảnh, receipt_id và kết quả đọc đều làm lại từ đầu.
 * Bấm "Chụp bill tiếp theo": bill đang chụp được gửi đọc ngay (chạy ngầm), màn chụp mở lại cho bill sau.
 */
export function Scan() {
  const [session, setSession] = useState(0);
  const [bg, setBg] = useState<BgBill | null>(null);
  const camera = useCamera();

  function readBg(receiptId: string, images: Blob[]) {
    setBg({ receiptId, images, status: "reading" });
    const same = (b: BgBill | null) => b?.receiptId === receiptId;
    apiUploadReceipt<ReceiptDraft>(receiptId, images, uiLang).then(
      (draft) => setBg((b) => (same(b) ? { ...b!, status: "ok", draft } : b)),
      (error) => setBg((b) => (same(b) ? { ...b!, status: "error", error } : b)),
    );
  }

  return (
    <ScanSession
      key={session}
      camera={camera}
      bg={bg}
      canAddBill={!bg && MAX_BILLS > 1}
      onNextBill={(receiptId, images) => {
        readBg(receiptId, images);
        setSession((s) => s + 1);
      }}
      onRetryBg={() => bg && readBg(bg.receiptId, bg.images)}
      onDropBg={() => setBg(null)}
      onNext={() => {
        setBg(null);
        setSession((s) => s + 1);
      }}
    />
  );
}

/**
 * Tạm thời (02/11, mẹ chốt): trên iPhone/iPad, trình duyệt hỏi lại quyền camera mỗi lần mở trang.
 * Ở đó nút chụp mở camera có sẵn của máy (<input capture>), không cần quyền. Khi có app cài
 * trên máy thì bỏ nhánh này. Android nhớ quyền theo trang nên vẫn dùng camera trong trang.
 */
const SYSTEM_CAMERA = typeof navigator !== "undefined" &&
  (/iPhone|iPad|iPod/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1));

/**
 * Mở camera MỘT lần cho cả màn quét, giữ luồng qua mọi bill và mọi lần gửi lỗi, tắt khi rời màn.
 * iPhone hỏi lại quyền camera mỗi lần gọi getUserMedia, nên không tắt rồi bật lại giữa chừng.
 */
function useCamera(): { stream: MediaStream | null; failed: boolean } {
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (SYSTEM_CAMERA) return; // iPhone: dùng camera của máy, không xin quyền
    let s: MediaStream | undefined;
    let stopped = false;
    if (!navigator.mediaDevices?.getUserMedia) {
      setFailed(true);
      return;
    }
    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: { ideal: "environment" }, width: { ideal: 1920 } }, audio: false })
      .then((ms) => {
        if (stopped) return ms.getTracks().forEach((t) => t.stop());
        s = ms;
        setStream(ms);
      })
      .catch(() => setFailed(true));
    return () => {
      stopped = true;
      s?.getTracks().forEach((t) => t.stop());
    };
  }, []);
  return { stream, failed };
}

/**
 * Màn quét (spec mục 4): nền tối toàn màn hình, nhiều ảnh cho một hóa đơn.
 * Hai nguồn ảnh:
 * - Camera: getUserMedia; máy không cho phép thì mở camera của hệ thống qua <input capture>.
 * - Thư viện ảnh: chọn ảnh hóa đơn đã chụp trước đó, chọn được nhiều ảnh một lần.
 *   Ô chọn thư viện KHÔNG có thuộc tính capture, vì capture buộc điện thoại mở camera.
 * Mọi ảnh được nén và bỏ EXIF ngay trên máy, chỉ nằm trong bộ nhớ trang tới khi gửi đi.
 */
function ScanSession({ camera, onNext, bg, canAddBill, onNextBill, onRetryBg, onDropBg }: {
  camera: { stream: MediaStream | null; failed: boolean };
  onNext: () => void;
  bg: BgBill | null;
  canAddBill: boolean;
  onNextBill: (receiptId: string, images: Blob[]) => void;
  onRetryBg: () => void;
  onDropBg: () => void;
}) {
  const nav = useNavigate();
  const videoRef = useRef<HTMLVideoElement>(null);
  const libraryRef = useRef<HTMLInputElement>(null);
  const cameraRef = useRef<HTMLInputElement>(null);
  const { stream, failed: cameraFailed } = camera;
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [busy, setBusy] = useState(false);
  // BUG-05 mục 8B: chụp lại riêng một ảnh lỗi, ảnh mới thay đúng vị trí (1-based).
  const [replaceAt, setReplaceAt] = useState<number | null>(null);
  const [receiptId] = useState(() => crypto.randomUUID()); // khóa idempotent cho cả lần đọc và lần lưu
  const read = useReadReceipt();
  // Xem lại: bill 1 (đọc ngầm) trước, rồi tới bill đang chụp. reviewOnly: bill này chưa chụp gì, chỉ xem bill 1.
  const [reviewOnly, setReviewOnly] = useState(false);
  const [bgSaved, setBgSaved] = useState(false);
  const urls = useRef<string[]>([]);

  // Gắn luồng camera vào thẻ video mỗi khi thẻ được tạo lại (sau màn chờ đọc) hoặc luồng mới có.
  useEffect(() => {
    const v = videoRef.current;
    if (v && stream && v.srcObject !== stream) {
      v.srcObject = stream;
      v.play().catch(() => {}); // iPhone đôi khi không tự chạy khi gán luồng mới
    }
  });

  // Thu hồi mọi URL xem trước khi rời màn.
  useEffect(() => () => urls.current.forEach((u) => URL.revokeObjectURL(u)), []);

  function push(blobs: Blob[], from: Photo["from"]) {
    const added = blobs.map((blob) => {
      const url = URL.createObjectURL(blob);
      urls.current.push(url);
      return { id: crypto.randomUUID(), blob, url, from };
    });
    if (replaceAt != null && added.length > 0) {
      const [first, ...rest] = added;
      setPhotos((p) => {
        const next = [...p];
        next.splice(replaceAt - 1, 1, first);
        return [...next, ...rest].slice(0, MAX_PHOTOS);
      });
      setReplaceAt(null);
      read.reset();
      return;
    }
    setPhotos((p) => [...p, ...added]);
  }

  async function addFiles(files: File[], from: Photo["from"]) {
    setNotice(null);
    const room = replaceAt != null ? 1 : MAX_PHOTOS - photos.length;
    if (room <= 0) return setNotice(t.scan.maxPhotos(MAX_PHOTOS));
    const picked = files.filter((f) => f.type.startsWith("image/") || f.type === "");
    const take = picked.slice(0, room);
    setBusy(true);
    const results = await Promise.allSettled(take.map(compressReceiptImage));
    setBusy(false);
    push(results.flatMap((r) => (r.status === "fulfilled" ? [r.value] : [])), from);

    const failed = results.filter((r) => r.status === "rejected").length + (files.length - picked.length);
    const skipped = picked.length - take.length;
    const parts = [];
    if (failed) parts.push(t.scan.unreadable(failed));
    if (skipped) parts.push(t.scan.skipped(skipped, MAX_PHOTOS));
    if (parts.length) setNotice(parts.join(" "));
  }

  async function shoot() {
    if (photos.length >= MAX_PHOTOS && replaceAt == null) return setNotice(t.scan.maxPhotos(MAX_PHOTOS));
    if (!videoRef.current || !stream) return cameraRef.current?.click();
    push([await grabFrame(videoRef.current)], "camera");
  }

  function remove(id: string) {
    setPhotos((p) => p.filter((x) => x.id !== id));
    setNotice(null);
  }

  function finish() {
    // Spec 1.5 mục 8: quét cần AI nên phải có mạng; mất mạng thì báo ngay, mời nhập thủ công.
    if (!navigator.onLine) return setOffline(true);
    // lang là ngôn ngữ dịch hóa đơn (tên món), không phải ngôn ngữ giao diện.
    // Gửi lại sau khi chụp lại một ảnh: backend nhớ kết quả các ảnh đã đọc được (theo nội dung
    // ảnh), nên chỉ ảnh mới được đọc lại (1.1).
    read.mutate({ receiptId, images: photos.map((p) => p.blob), lang: uiLang }); // camera vẫn mở để chụp lại ngay
  }

  // 1.1: lỗi gắn với một ảnh (unreadable_image, image_too_large, read_failed) chỉ ra đúng ảnh cần chụp lại.
  const badImages = new Set<number>();
  if (read.error instanceof ApiError) {
    const f = read.error.fields;
    if (Array.isArray(f.image_indexes)) f.image_indexes.forEach((i) => typeof i === "number" && badImages.add(i));
    else if (typeof f.image_index === "number") badImages.add(f.image_index);
  }

  const ownReady = reviewOnly || !!read.data;
  if (ownReady && bg && !bgSaved) {
    if (bg.status === "reading") return <Waiting text={m15.scan.waitBg} />;
    if (bg.status === "error") {
      const msg = bg.error instanceof OfflineError ? t.scan.uploadOffline : bg.error instanceof Error ? bg.error.message : "";
      return (
        <main className="scan">
          <div className="scan__view" style={{ flex: 1, height: "auto" }}>
            <div className="scan__fallback">
              <p role="alert">{m15.scan.bgError(msg)}</p>
              <button type="button" className="btn btn--primary" onClick={onRetryBg}>{m15.scan.retryBg}</button>
              <button type="button" className="btn btn--quiet" onClick={() => (reviewOnly ? onNext() : onDropBg())}>{m15.scan.skipBg}</button>
            </div>
          </div>
        </main>
      );
    }
    if (bg.draft) {
      return (
        <Confirm
          key={bg.receiptId}
          draft={bg.draft}
          onDone={() => nav("/")}
          onNext={onNext}
          onContinue={reviewOnly ? undefined : () => setBgSaved(true)}
        />
      );
    }
  }

  if (read.isPending) {
    return (
      <main className="scan" aria-busy="true">
        <div className="scan__view">
          <div style={{ display: "grid", justifyItems: "center", gap: 16 }}>
            <MimiMark size={88} animated />
            <p className="scan__hint" role="status">{t.scan.reading(photos.length)}</p>
          </div>
        </div>
      </main>
    );
  }

  if (read.data) return <Confirm key={receiptId} draft={read.data} onDone={() => nav("/")} onNext={onNext} />;
  if (reviewOnly) return <Waiting text={m15.scan.waitBg} />; // bill 1 lưu xong: onNext đã mở phiên mới

  const fromLibrary = photos.some((p) => p.from === "library");

  return (
    <main className="scan">
      <div className="scan__top">
        <Link to="/" className="icon-btn" aria-label={t.scan.close}><X size={26} aria-hidden /></Link>
        <Link to="/" className="icon-btn" aria-label={t.nav.home}><MimiMark size={36} label={null} /></Link>
      </div>

      <p className="scan__privacy" role="note">⚠️ {m15.scan.privacy}</p>

      <div className="scan__view">
        {SYSTEM_CAMERA ? (
          photos.length > 0 ? (
            <img className="scan__last" src={photos[photos.length - 1].url} alt={t.scan.photoAlt(photos.length, false)} />
          ) : (
            <button type="button" className="scan__fallback scan__system" onClick={() => cameraRef.current?.click()}>
              <Camera size={48} aria-hidden />
              <span>{m15.scan.systemCamera}</span>
            </button>
          )
        ) : !cameraFailed ? (
          <video ref={videoRef} autoPlay playsInline muted aria-label={t.scan.camera} />
        ) : (
          <div className="scan__fallback">
            <p>{t.scan.cameraFailed}</p>
            <button type="button" className="btn btn--primary" onClick={() => libraryRef.current?.click()}>
              <ImagePlus size={20} aria-hidden /> {t.scan.pickSaved}
            </button>
            <button type="button" className="btn btn--quiet" onClick={() => cameraRef.current?.click()}>
              <Camera size={20} aria-hidden /> {t.scan.openCamera}
            </button>
          </div>
        )}
        {bg && (
          <p className="scan__badge" role="status">
            {bg.status === "reading" ? m15.scan.bgReading : bg.status === "ok" ? m15.scan.bgDone : m15.scan.bgFailed}
          </p>
        )}
      </div>

      {/* Có ảnh rồi thì bỏ lời dặn để nút "Xong" không bị đẩy khỏi màn hình */}
      {photos.length === 0 && <p className="scan__hint">{fromLibrary ? t.scan.hintLibrary : t.scan.hintCamera}</p>}

      {photos.length > 0 && (
        <ul className="scan__thumbs" aria-label={t.scan.photosOfReceipt(photos.length)}>
          {photos.map((p, i) => (
            <li key={p.id} style={badImages.has(i + 1) || replaceAt === i + 1 ? { outline: "3px solid var(--danger, #c0392b)", outlineOffset: 2 } : undefined}>
              <img src={p.url} alt={t.scan.photoAlt(i + 1, p.from === "library")} />
              {badImages.has(i + 1) && <span className="sr-only">{m1.scan.badImage}</span>}
              <button type="button" className="thumb__remove" aria-label={t.scan.removePhoto(i + 1)} onClick={() => remove(p.id)}>
                <X size={14} aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}

      <div aria-live="polite">
        {busy && <p className="scan__hint">{t.scan.preparing}</p>}
        {notice && <p className="scan__hint scan__notice">{notice}</p>}
        {read.isError && replaceAt == null && (
          <>
            <p className="scan__hint scan__notice" role="alert">
              {read.error instanceof OfflineError ? t.scan.uploadOffline : read.error.message}
            </p>
            <div className="plan-form__row" style={{ justifyContent: "center" }}>
              {[...badImages].filter((i) => i <= photos.length).map((i) => (
                <button key={i} type="button" className="btn btn--quiet" onClick={() => setReplaceAt(i)}>
                  {m1.scan.retakeImage(i)}
                </button>
              ))}
            </div>
          </>
        )}
        {replaceAt != null && (
          <div className="plan-form__row" style={{ justifyContent: "center", alignItems: "center" }}>
            <p className="scan__hint">{m1.scan.retakeNow(replaceAt)}</p>
            <button type="button" className="btn btn--quiet" onClick={() => setReplaceAt(null)}>{m1.scan.cancelRetake}</button>
          </div>
        )}
      </div>

      <div className="scan__controls">
        <button type="button" className="btn btn--quiet scan__lib" onClick={() => libraryRef.current?.click()}>
          <ImagePlus size={20} aria-hidden /> {t.scan.library}
        </button>
        <button type="button" className="shutter" aria-label={t.scan.shutter} onClick={shoot} />
        {canAddBill && photos.length > 0 && !read.isError ? (
          // Chụp xong bill này: gửi đọc ngầm, mở màn chụp cho bill sau
          <button
            type="button"
            className="btn btn--quiet scan__next"
            aria-disabled={busy || undefined}
            onClick={() => {
              if (busy) return;
              if (!navigator.onLine) return setOffline(true);
              onNextBill(receiptId, photos.map((p) => p.blob));
            }}
          >
            {m15.scan.nextBill} <ChevronRight size={18} aria-hidden />
          </button>
        ) : (
          <span className="scan__count">{photos.length > 0 ? t.scan.count(photos.length, MAX_PHOTOS) : ""}</span>
        )}
      </div>

      {/* Thư viện: không có capture, chọn nhiều ảnh */}
      <input
        ref={libraryRef}
        type="file"
        accept="image/*"
        multiple
        hidden
        onChange={(e) => {
          addFiles([...(e.target.files ?? [])], "library");
          e.target.value = "";
        }}
      />
      {/* Camera hệ thống: dự phòng khi getUserMedia không dùng được */}
      <input
        ref={cameraRef}
        type="file"
        accept="image/*"
        capture="environment"
        hidden
        onChange={(e) => {
          addFiles([...(e.target.files ?? [])], "camera");
          e.target.value = "";
        }}
      />

      {offline && (
        <div className="run15__offline scan15__offline" role="alert">
          <p>{m15.run.offline}</p>
          <button type="button" className="btn btn--outline" onClick={() => nav("/plan")}>{m15.run.manual}</button>
        </div>
      )}
      {(photos.length > 0 || bg) && (
        <div className="scan__done">
          {photos.length > 0 ? (
            <button type="button" className="btn btn--primary btn--block btn--lg" onClick={finish} aria-disabled={busy || undefined}>
              {t.scan.finish(photos.length)}
            </button>
          ) : (
            <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => setReviewOnly(true)}>
              {m15.scan.reviewOnly}
            </button>
          )}
        </div>
      )}
    </main>
  );
}

function Waiting({ text }: { text: string }) {
  return (
    <main className="scan" aria-busy="true">
      <div className="scan__view">
        <div style={{ display: "grid", justifyItems: "center", gap: 16 }}>
          <MimiMark size={88} animated />
          <p className="scan__hint" role="status">{text}</p>
        </div>
      </div>
    </main>
  );
}

/** YYYY-MM-DD theo giờ máy người dùng (ngày mua mặc định khi hóa đơn không in ngày). */
function todayLocal(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/**
 * Xác nhận hóa đơn: màn tối giản, đối chiếu tổng tiền thành một dòng chữ.
 * Mọi con số (từng dòng, tổng, khớp hay lệch) do backend tính trong bản nháp; khi lưu backend tính lại.
 */
function Confirm({ draft, onDone, onNext, onContinue }: {
  draft: ReceiptDraft;
  onDone: () => void;
  onNext: () => void;
  /** Còn bill sau cần xem: nút chính sau khi lưu chuyển sang bill đó. */
  onContinue?: () => void;
}) {
  const [usePrediction, setUsePrediction] = useState(true);
  const [purchasedOn, setPurchasedOn] = useState(draft.purchased_on || todayLocal());
  const save = useSaveReceipt();
  // 4.1: Buyer chọn hóa đơn này cho vòng mua hộ nào (hoặc của riêng mình).
  const delegated = useDelegated();
  const toMe = delegated.data?.to_me ?? [];
  const [assignmentId, setAssignmentId] = useState("");
  // Spec 1.5 mục 3: mỗi món trừ vào kế hoạch có ngày sớm nhất trước; người dùng đổi được.
  const plans = visiblePlans(usePlans().data);
  const carts = usePlanCarts(plans.map((p) => p.id)).map((q) => q.data);
  const [override, setOverride] = useState<Record<string, string>>({});
  const [pickPlan, setPickPlan] = useState<string | null>(null);
  // Bản nháp người dùng sửa: lỗi lưu không làm mất chỉnh sửa (BUG-06 mục 10).
  const [lines, setLines] = useState<EditLine[]>(draft.lines);
  const [localError, setLocalError] = useState("");
  const edited = lines !== draft.lines;
  const kept = lines.filter((l) => !l.removed);
  const review = kept.filter((l) => l.needs_review && !l.discount).length;
  const noPrice = kept.filter((l) => !l.discount && l.amount.trim() === "").length;
  const sum = edited ? kept.reduce((a, l) => a + (l.amount_vnd ?? 0), 0) : draft.sum_vnd;
  const printed = draft.total_printed ? parseAmount(draft.total_printed, draft.currency) : null;
  // Tổng lệch chỉ là cảnh báo, không chặn lưu (BUG-06 mục 5.1).
  const mismatch = edited && draft.currency === "VND" && printed != null ? Math.abs(printed - sum) > 1000 : draft.total_mismatch;
  const matches = !!draft.total_printed && !mismatch;
  const alloc = allocate(kept.map((l) => ({ id: l.line_id, key: l.group_key, qty: l.qty })), planNeeds(plans, carts, planName));
  const planLine = (l: EditLine) => {
    const o = override[l.line_id];
    const text = o === "none"
      ? m15.scanPlan.none
      : o
        ? `→ ${planName(plans.find((p) => p.id === o) ?? { default: false, name: "" })}`
        : (alloc.get(l.line_id) ?? []).length
          ? `→ ${(alloc.get(l.line_id) ?? []).map((a, _i, all) => (all.length > 1 ? `${a.name} (${a.qty})` : a.name)).join(", ")}`
          : m15.scanPlan.none;
    return (
      <span className="scan15__plan">
        <button type="button" className="link15" aria-expanded={pickPlan === l.line_id} onClick={() => setPickPlan(pickPlan === l.line_id ? null : l.line_id)}>{text}</button>
        {pickPlan === l.line_id && (
          <select
            aria-label={m15.scanPlan.choose}
            value={o ?? ""}
            onChange={(e) => {
              setOverride((m) => ({ ...m, [l.line_id]: e.target.value }));
              setPickPlan(null);
            }}
          >
            <option value="">{m15.scanPlan.auto}</option>
            {plans.map((p) => <option key={p.id} value={p.id}>{planName(p)}</option>)}
            <option value="none">{m15.scanPlan.noneOpt}</option>
          </select>
        )}
      </span>
    );
  };
  const change = (id: string, patch: Partial<EditLine>) => {
    setLocalError("");
    setLines((ls) => ls.map((l) => (l.line_id === id ? { ...l, ...patch } : l)));
  };
  const submit = (confirmDuplicate: boolean) => {
    if (save.isPending) return;
    if (kept.length === 0) return setLocalError(m1.receiptEdit.keepOne);
    // B3: gửi đúng phần chia đang hiện cho từng món (tự chia, hoặc kế hoạch người dùng chọn, hoặc không trừ).
    const linePlans = new Map<string, { plan_id: string; qty: number }[]>();
    if (!assignmentId) {
      for (const l of kept) {
        if (l.discount || !l.group_key) continue;
        const o = override[l.line_id];
        linePlans.set(l.line_id, o === "none" ? [] : o ? [{ plan_id: o, qty: l.qty }]
          : (alloc.get(l.line_id) ?? []).map((a) => ({ plan_id: a.planId, qty: a.qty })));
      }
    }
    save.mutate(toSaveRequest(draft, { purchasedOn, usePrediction, confirmDuplicate, planId: currentPlanId(), assignmentId, linePlans }, kept));
  };

  if (save.isSuccess) {
    return (
      <main className="frame">
        <div className="page done">
          <div className="done__head">
            <MimiMark size={64} />
            <h1>{t.confirm.savedTitle}</h1>
            <p>{t.confirm.savedBody(kept.length, usePrediction)}</p>
            {save.data?.assignment && (
              <p>{m1.scan.assignmentResult(save.data.assignment.done.length, save.data.assignment.returned.length)}</p>
            )}
            {!!save.data?.plan_done?.length && <p>{m1.scan.planDone(save.data.plan_done.length)}</p>}
            {save.data?.split_plan && (
              <p><Link to={`/plan/${encodeURIComponent(save.data.split_plan.plan_id)}`}>{m1.scan.splitPlan(save.data.split_plan.moved)} ›</Link></p>
            )}
            {!!save.data?.recurring_matched?.length && <p>{m1.scan.recurringMatched}</p>}
          </div>
          <dl className="facts">
            <dt>{t.confirm.store}</dt>
            <dd>{draft.store || t.confirm.unknown}</dd>
            <dt>{t.confirm.date}</dt>
            <dd>{formatDayMonth(purchasedOn)}</dd>
            <dt>{t.confirm.total}</dt>
            <dd className="num">{formatVnd(sum)}</dd>
          </dl>
          {/* Cùng quy cách với nút "Lưu hóa đơn": nút chính rộng hết khung, nút phụ viền nhạt bên dưới */}
          <div className="actions">
            {onContinue ? (
              <button type="button" className="btn btn--primary btn--block btn--lg" onClick={onContinue}>
                {m15.scan.seeNext}
              </button>
            ) : (
              <button type="button" className="btn btn--primary btn--block btn--lg" onClick={onNext}>
                {t.confirm.scanAnother}
              </button>
            )}
            <button type="button" className="btn btn--quiet btn--block" onClick={onDone}>
              {t.common.backHome}
            </button>
          </div>
        </div>
      </main>
    );
  }

  // BUG-06 mục 8: mỗi loại lỗi một cách báo; lỗi ngày tô ô ngày, lỗi một dòng tô đúng dòng đó.
  const err = save.error;
  const duplicate = err instanceof ApiError && err.code === "possible_duplicate";
  const f = err instanceof ApiError ? err.fields : {};
  const badLine = typeof f.line_id === "string" ? f.line_id : undefined;
  const badDate = err instanceof ApiError && (err.code === "invalid_purchase_date" || f.field === "purchased_on");
  const lineName = lines.find((l) => l.line_id === badLine)?.name ?? "";
  const errorText = localError || (!err
    ? ""
    : err instanceof OfflineError
      ? `${t.confirm.saveFailed} ${t.confirm.checkConnection}`
      : duplicate
        ? t.confirm.duplicate
        : badDate
          ? m1.receiptEdit.fixDate
          : badLine
            ? `${err.message} ${m1.receiptEdit.fixLine(lineName)}`
            : err instanceof ApiError && err.status < 500 && !err.retryable
              ? err.message
              : m1.receiptEdit.serverError);

  return (
    <main className="frame">
      <div className="page">
        <div className="confirm__head">
          {draft.store || t.confirm.noStore}
          <label className="confirm__date">
            <span className="sr-only">{t.confirm.date}</span>
            <input
              type="date"
              value={purchasedOn}
              max={todayLocal()}
              aria-invalid={badDate || undefined}
              style={badDate ? { outline: "2px solid var(--st-out)" } : undefined}
              onChange={(e) => e.target.value && setPurchasedOn(e.target.value)}
            />
          </label>
        </div>
        {draft.unreadable_pages > 0 && <p className="check warn">{t.confirm.unreadablePages(draft.unreadable_pages)}</p>}
        <ReceiptLines lines={lines} currency={draft.currency} badLine={badLine} onChange={change} extra={planLine} />
        <p className={`check ${matches ? "ok" : "warn"}`}>
          {!draft.total_printed
            ? t.confirm.noTotal(formatVnd(sum))
            : matches
              ? t.confirm.matches(formatVnd(sum))
              : t.confirm.mismatch(formatVnd(sum), draft.total_printed)}
        </p>
        {review > 0 && <p className="check warn">{m1.scan.reviewHint(review)}</p>}
        {noPrice > 0 && <p className="check">{t.confirm.noPriceCount(noPrice)}</p>}
        {toMe.length > 0 && (
          <label className="field">
            {m1.scan.forAssignment}
            <select value={assignmentId} onChange={(e) => setAssignmentId(e.target.value)}>
              <option value="">{m1.scan.personal}</option>
              {toMe.map((a) => (
                <option key={a.assignment_id} value={a.assignment_id}>
                  {m1.delegated.from(a.owner_display_name || m1.assign.someone)}
                </option>
              ))}
            </select>
          </label>
        )}
        <label className="switch-row">
          {t.confirm.usePrediction}
          <input type="checkbox" role="switch" {...{ switch: "" }} checked={usePrediction} onChange={(e) => setUsePrediction(e.target.checked)} />
        </label>
        {errorText && <p className="error" role="alert">{errorText}</p>}
        {duplicate ? (
          <button className="btn btn--primary btn--block btn--lg" aria-disabled={save.isPending || undefined} onClick={() => submit(true)}>
            {save.isPending ? t.confirm.saving : t.confirm.saveAnyway}
          </button>
        ) : (
          <button className="btn btn--primary btn--block btn--lg" aria-disabled={save.isPending || undefined} onClick={() => submit(false)}>
            {save.isPending ? t.confirm.saving : t.confirm.save}
          </button>
        )}
      </div>
    </main>
  );
}
