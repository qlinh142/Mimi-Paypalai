import { t, uiLang } from "../i18n";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Camera, ImagePlus, X } from "lucide-react";
import { toSaveRequest, useReadReceipt, useSaveReceipt } from "../api/hooks";
import type { ReceiptDraft } from "../api/types";
import { ApiError, OfflineError } from "../api/client";
import { compressReceiptImage, grabFrame } from "../lib/image";
import { formatDayMonth, formatVnd } from "../lib/text";
import { MimiMark } from "../components/MimiMark";

const MAX_PHOTOS = 8;

type Photo = { id: string; blob: Blob; url: string; from: "camera" | "library" };

/** Mỗi lần "Quét hóa đơn khác" tạo phiên mới: ảnh, receipt_id và kết quả đọc đều làm lại từ đầu. */
export function Scan() {
  const [session, setSession] = useState(0);
  return <ScanSession key={session} onNext={() => setSession((s) => s + 1)} />;
}

/**
 * Màn quét (spec mục 4): nền tối toàn màn hình, nhiều ảnh cho một hóa đơn.
 * Hai nguồn ảnh:
 * - Camera: getUserMedia; máy không cho phép thì mở camera của hệ thống qua <input capture>.
 * - Thư viện ảnh: chọn ảnh hóa đơn đã chụp trước đó, chọn được nhiều ảnh một lần.
 *   Ô chọn thư viện KHÔNG có thuộc tính capture, vì capture buộc điện thoại mở camera.
 * Mọi ảnh được nén và bỏ EXIF ngay trên máy, chỉ nằm trong bộ nhớ trang tới khi gửi đi.
 */
function ScanSession({ onNext }: { onNext: () => void }) {
  const nav = useNavigate();
  const videoRef = useRef<HTMLVideoElement>(null);
  const libraryRef = useRef<HTMLInputElement>(null);
  const cameraRef = useRef<HTMLInputElement>(null);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [cameraFailed, setCameraFailed] = useState(false);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [receiptId] = useState(() => crypto.randomUUID()); // khóa idempotent cho cả lần đọc và lần lưu
  const read = useReadReceipt();
  const urls = useRef<string[]>([]);

  useEffect(() => {
    let s: MediaStream | undefined;
    if (!navigator.mediaDevices?.getUserMedia) {
      setCameraFailed(true);
      return;
    }
    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: { ideal: "environment" }, width: { ideal: 1920 } }, audio: false })
      .then((ms) => {
        s = ms;
        setStream(ms);
        if (videoRef.current) videoRef.current.srcObject = ms;
      })
      .catch(() => setCameraFailed(true));
    return () => s?.getTracks().forEach((t) => t.stop());
  }, []);

  // Thu hồi mọi URL xem trước khi rời màn.
  useEffect(() => () => urls.current.forEach((u) => URL.revokeObjectURL(u)), []);

  function push(blobs: Blob[], from: Photo["from"]) {
    const added = blobs.map((blob) => {
      const url = URL.createObjectURL(blob);
      urls.current.push(url);
      return { id: crypto.randomUUID(), blob, url, from };
    });
    setPhotos((p) => [...p, ...added]);
  }

  async function addFiles(files: File[], from: Photo["from"]) {
    setNotice(null);
    const room = MAX_PHOTOS - photos.length;
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
    if (photos.length >= MAX_PHOTOS) return setNotice(t.scan.maxPhotos(MAX_PHOTOS));
    if (!videoRef.current || !stream) return cameraRef.current?.click();
    push([await grabFrame(videoRef.current)], "camera");
  }

  function remove(id: string) {
    setPhotos((p) => p.filter((x) => x.id !== id));
    setNotice(null);
  }

  function finish() {
    stream?.getTracks().forEach((t) => t.stop());
    // lang là ngôn ngữ dịch hóa đơn (tên món), không phải ngôn ngữ giao diện
    read.mutate({ receiptId, images: photos.map((p) => p.blob), lang: uiLang });
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

  if (read.data) return <Confirm draft={read.data} onDone={() => nav("/")} onNext={onNext} />;

  const fromLibrary = photos.some((p) => p.from === "library");

  return (
    <main className="scan">
      <div className="scan__top">
        <Link to="/" className="icon-btn" aria-label={t.scan.close}><X size={26} aria-hidden /></Link>
        <Link to="/" className="icon-btn" aria-label={t.nav.home}><MimiMark size={36} label={null} /></Link>
      </div>

      <div className="scan__view">
        {!cameraFailed ? (
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
      </div>

      <p className="scan__hint">
        {fromLibrary
          ? t.scan.hintLibrary
          : t.scan.hintCamera}
      </p>

      {photos.length > 0 && (
        <ul className="scan__thumbs" aria-label={t.scan.photosOfReceipt(photos.length)}>
          {photos.map((p, i) => (
            <li key={p.id}>
              <img src={p.url} alt={t.scan.photoAlt(i + 1, p.from === "library")} />
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
        {read.isError && (
          <p className="scan__hint scan__notice" role="alert">
            {read.error instanceof OfflineError ? t.scan.uploadOffline : read.error.message}
          </p>
        )}
      </div>

      <div className="scan__controls">
        <button type="button" className="btn btn--quiet scan__lib" onClick={() => libraryRef.current?.click()}>
          <ImagePlus size={20} aria-hidden /> {t.scan.library}
        </button>
        <button type="button" className="shutter" aria-label={t.scan.shutter} onClick={shoot} />
        <span className="scan__count">{photos.length > 0 ? t.scan.count(photos.length, MAX_PHOTOS) : ""}</span>
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

      {photos.length > 0 && (
        <div className="scan__done">
          <button type="button" className="btn btn--primary btn--block btn--lg" onClick={finish} aria-disabled={busy || undefined}>
            {t.scan.finish(photos.length)}
          </button>
        </div>
      )}
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
function Confirm({ draft, onDone, onNext }: { draft: ReceiptDraft; onDone: () => void; onNext: () => void }) {
  const [usePrediction, setUsePrediction] = useState(true);
  const [purchasedOn, setPurchasedOn] = useState(draft.purchased_on || todayLocal());
  const save = useSaveReceipt();
  const sum = draft.sum_vnd;
  const matches = !!draft.total_printed && !draft.total_mismatch;
  const submit = (confirmDuplicate: boolean) =>
    !save.isPending &&
    save.mutate(toSaveRequest(draft, { purchasedOn, usePrediction, confirmDuplicate }));

  if (save.isSuccess) {
    return (
      <main className="frame">
        <div className="page done">
          <div className="done__head">
            <MimiMark size={64} />
            <h1>{t.confirm.savedTitle}</h1>
            <p>{t.confirm.savedBody(draft.lines.length, usePrediction)}</p>
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
            <button type="button" className="btn btn--primary btn--block btn--lg" onClick={onNext}>
              {t.confirm.scanAnother}
            </button>
            <button type="button" className="btn btn--quiet btn--block" onClick={onDone}>
              {t.common.backHome}
            </button>
          </div>
        </div>
      </main>
    );
  }

  const err = save.error;
  const duplicate = err instanceof ApiError && err.code === "possible_duplicate";
  const errorText = !err
    ? ""
    : err instanceof OfflineError
      ? `${t.confirm.saveFailed} ${t.confirm.checkConnection}`
      : duplicate
        ? t.confirm.duplicate
        : err instanceof ApiError && err.code === "validation_failed"
          ? `${t.confirm.saveFailed} ${t.confirm.checkDetails}`
          : `${t.confirm.saveFailed} ${t.common.genericError}`;

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
              onChange={(e) => e.target.value && setPurchasedOn(e.target.value)}
            />
          </label>
        </div>
        {draft.unreadable_pages > 0 && <p className="check warn">{t.confirm.unreadablePages(draft.unreadable_pages)}</p>}
        <ul className="lines">
          {draft.lines.map((l) => (
            <li key={l.line_id}>
              <span>{l.name}</span>
              <span className="num">{l.amount_vnd != null ? formatVnd(l.amount_vnd) : `${l.amount} ${draft.currency}`}</span>
              <span className="sub">
                {l.discount ? t.confirm.discountLine : t.confirm.qty(l.qty)}
                {!l.discount && l.unit && ` ${l.unit}`}
                {l.amount_vnd == null && ` · ${t.confirm.notConverted}`}
              </span>
              {!l.category_key && !l.discount && <button className="btn btn--quiet" style={{ minHeight: 36, gridColumn: 2 }}>{t.confirm.chooseGroup}</button>}
            </li>
          ))}
        </ul>
        <p className={`check ${matches ? "ok" : "warn"}`}>
          {!draft.total_printed
            ? t.confirm.noTotal(formatVnd(sum))
            : matches
              ? t.confirm.matches(formatVnd(sum))
              : t.confirm.mismatch(formatVnd(sum), draft.total_printed)}
        </p>
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
