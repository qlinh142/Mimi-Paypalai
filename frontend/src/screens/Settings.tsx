import { setHideAmounts, useHideAmounts } from "../lib/hideAmounts";
import { s22 } from "../i18n/s22";
import { switchUiLang, t, uiLang } from "../i18n";
import { displayCurrency, switchCurrency, type DisplayCurrency } from "../lib/money";
import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Bell, ChevronRight, Copy, Globe, Info, LogOut, Package, Plus, ShieldCheck, UsersRound, Wallet,
  type LucideIcon,
} from "lucide-react";
import { ApiError } from "../api/client";
import { useDeleteAccount, useHousehold, useHouseholdActions, useMandate, useMe, useSaveMandate, useSaveSettings } from "../api/hooks";
import type { Country, Mandate, MandateCategory, Me } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { m15 } from "../i18n/m15";
import { m1 } from "../i18n/m1";
import { useAuth } from "../lib/auth";
import { useQueue } from "../lib/offlineQueue";
import { flushQueue } from "../api/client";
import { RELATIONS, useRelations } from "../lib/relation";
import { wipeDeviceData } from "../lib/localData";

const S = m15.settings;
const LEAD_DAYS = [1, 2, 3, 5, 7, 10, 14, 21, 30];
const TIMEZONES = [
  "Asia/Ho_Chi_Minh", "Asia/Bangkok", "Asia/Singapore", "Asia/Tokyo", "Asia/Seoul",
  "Australia/Sydney", "Europe/London", "Europe/Berlin", "America/New_York", "America/Los_Angeles",
];
// Nước cho chọn trong Cài đặt (mẹ chốt 03/11: Việt Nam, Mỹ), khớp market.Selectable ở máy chủ.
// Người đã chọn nước khác trước đó (Singapore) vẫn thấy nước của mình trong danh sách.
const COUNTRIES: Country[] = ["VN", "US"];
// Chưa chọn nước thì đoán theo múi giờ, khớp market.CountryFor ở máy chủ.
const ZONE_COUNTRY: Record<string, Country> = {
  "America/New_York": "US", "America/Chicago": "US", "America/Denver": "US", "America/Phoenix": "US",
  "America/Los_Angeles": "US", "America/Anchorage": "US", "Pacific/Honolulu": "US", "Asia/Singapore": "SG",
};
const LANG_NAME = { vi: "Tiếng Việt", en: "English" } as const;
const initial = (n: string) => (n.trim()[0] ?? "?").toUpperCase();
const errText = (e: unknown) => (e instanceof ApiError ? e.message : t.common.genericError);

/** Một dòng trong danh sách Cài đặt. `soon`: tính năng đang treo (Mimi 1.5), làm mờ, bấm chỉ báo Sắp ra mắt. */
function Row({ to, Icon, label, sub, chip, soon, onSoon }: { to?: string; Icon: LucideIcon; label: string; sub?: string; chip?: string; soon?: boolean; onSoon?: () => void }) {
  const body = (
    <>
      <span className="set15__icon"><Icon size={22} aria-hidden /></span>
      <span className="set15__text">
        <span>{label}</span>
        {sub && <small>{sub}</small>}
      </span>
      {chip && <span className="tag15 set15__chip">{chip}</span>}
      {!soon && <ChevronRight size={20} aria-hidden />}
    </>
  );
  if (soon || !to)
    return (
      <li>
        <button type="button" className={`set15__row${soon ? " set15__row--soon" : ""}`} aria-disabled={soon || undefined} onClick={onSoon}>{body}</button>
      </li>
    );
  return (
    <li>
      <Link to={to} className="set15__row">{body}</Link>
    </li>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="set15__group" aria-label={title}>
      <h2 className="h15sec">{title}</h2>
      <ul className="box15 set15__list">{children}</ul>
    </section>
  );
}

/** Khung chung cho các trang con của Cài đặt: nút ‹ quay lại và tên trang. */
function Sub({ title, children }: { title: string; children: ReactNode }) {
  return (
    <MainFrame>
      <TopBar title={title} variant="sub" />
      <div className="page p15 set15">{children}</div>
    </MainFrame>
  );
}

function useMeData(): Me | undefined {
  return useMe().data;
}

/** Cài đặt (Mimi 1.5, ảnh mẫu 6-cai-dat-va-thanh-vien.png, chốt 31/10 14:46). */
export function Settings() {
  const me = useMeData();
  const auth = useAuth();
  const h = useHousehold();
  const s = me?.settings;
  const name = me?.display_name || auth.session?.email || "";
  return (
    <MainFrame>
      <TopBar title={S.title} sub={s22.heads.settings} showTitle />
      <div className="page p15 set15">
        <Link to="/settings/profile" className="box15 set15__me">
          <span className="set15__avatar" aria-hidden>{initial(name)}</span>
          <span className="set15__text">
            <strong>{name}</strong>
            <small>{S.seeProfile}</small>
          </span>
          <ChevronRight size={22} aria-hidden />
        </Link>

        <SyncBox />

        <Group title={S.general}>
          {/* Anh chốt 09/11 (T2, T5): ngưỡng cố định 3 và 7 ngày, chưa nhắc qua email. */}
          <Row to="/settings/notify" Icon={Bell} label={S.notify} sub={S.notifyFixedSub} />
          <Row to="/settings/language" Icon={Globe} label={S.language} sub={S.languageSub(LANG_NAME[uiLang], (s?.timezone || "Asia/Ho_Chi_Minh").replace(/_/g, " "))} />
        </Group>
        <p className="hint hint15"><Info size={16} aria-hidden /> {S.langNote}</p>

        <Group title={S.family}>
          <Row to="/settings/members" Icon={UsersRound} label={S.members} sub={h.data ? S.membersSub(h.data.members.length) : undefined} />
          {/* Anh chốt 09/11 (T1): bỏ Ủy quyền mua online khỏi Cài đặt; Mimi chỉ đề xuất so sánh giá. Backend giữ nguyên. */}
        </Group>

        <Group title={S.shopping}>
          <Row to="/orders" Icon={Package} label={S.orders} />
        </Group>

        <Group title={S.account}>
          <Row to="/settings/privacy" Icon={ShieldCheck} label={S.privacy} />
          <li>
            <button
              type="button"
              className="set15__row set15__row--danger"
              onClick={async () => {
                await wipeDeviceData();
                await auth.signOut();
              }}
            >
              <span className="set15__icon"><LogOut size={22} aria-hidden /></span>
              <span className="set15__text"><span>{S.signOut}</span></span>
            </button>
          </li>
        </Group>
      </div>
    </MainFrame>
  );
}

function SaveBar({ dirty, pending, ok, err }: { dirty: boolean; pending: boolean; ok: boolean; err: boolean }) {
  return (
    <>
      {err && <p className="error" role="alert">{t.settings.saveFailed}</p>}
      {ok && !dirty && <p className="hint" role="status">{S.saved}</p>}
      <button type="submit" className="btn btn--primary btn--block btn--lg" aria-disabled={!dirty || pending || undefined}>
        {pending ? t.confirm.saving : S.save}
      </button>
    </>
  );
}

/** Hồ sơ cá nhân: sửa tên Mimi gọi; email chỉ xem. */
export function SettingsProfile() {
  const me = useMeData();
  const auth = useAuth();
  const save = useSaveSettings();
  const [name, setName] = useState<string | null>(null);
  const cur = name ?? me?.display_name ?? "";
  const dirty = name != null && name.trim() !== "" && name.trim() !== (me?.display_name ?? "");
  return (
    <Sub title={S.profile}>
      <form
        className="box15 set15__form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!dirty || save.isPending) return;
          save.mutate({ display_name: cur.trim() }, { onSuccess: () => setName(null) });
        }}
      >
        <span className="set15__avatar set15__avatar--lg" aria-hidden>{initial(cur || auth.session?.email || "")}</span>
        {/* B1 (theo Redesign): email và vai trò dưới avatar. */}
        <p className="set15__who">
          <span>{auth.session?.email ?? ""}</span>
          {me && <span className="tag15">{me.role === "member" ? S.member : S.owner}</span>}
        </p>
        <label className="field15">
          <span>{S.name}</span>
          <input value={cur} maxLength={40} autoComplete="given-name" onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="field15">
          <span>{S.email}</span>
          <input value={auth.session?.email ?? ""} readOnly aria-readonly="true" className="set15__ro" />
        </label>
        <SaveBar dirty={dirty} pending={save.isPending} ok={save.isSuccess} err={save.isError} />
      </form>
    </Sub>
  );
}

/**
 * Thông báo & nhắc mua. Anh chốt 09/11 (T2, T5): nhắc trong app theo 2 ngưỡng cố định (Cần mua ngay
 * ≤ 3 ngày, Sắp hết ≤ 7 ngày), chưa gửi email nên không còn công tắc, giờ gửi, số ngày báo trước.
 * NotifyForm của 2.1 giữ lại để bật lại khi làm nhắc qua email.
 */
export function SettingsNotify() {
  return (
    <Sub title={S.notify}>
      <div className="box15 set15__form">
        <p>{S.notifyInApp}</p>
        <ul className="set15__rules">
          <li><span className="set15__dot set15__dot--now" aria-hidden /> {S.notifyNow}</li>
          <li><span className="set15__dot set15__dot--soon" aria-hidden /> {S.notifySoon}</li>
        </ul>
        <p className="hint">{S.notifyNoEmail}</p>
      </div>
    </Sub>
  );
}

export function NotifyForm({ me }: { me: Me }) {
  const s = me.settings;
  const start = { on: s.remind_enabled ?? true, time: s.digest_time ?? "08:00", lead: s.lead_days ?? 7 };
  const [base, setBase] = useState(start);
  const [f, setF] = useState(start);
  const save = useSaveSettings();
  const dirty = f.on !== base.on || f.time !== base.time || f.lead !== base.lead;
  return (
    <form
      className="box15 set15__form"
      onSubmit={(e) => {
        e.preventDefault();
        if (!dirty || save.isPending) return;
        save.mutate({ remind_enabled: f.on, digest_time: f.time, lead_days: f.lead }, { onSuccess: () => setBase(f) });
      }}
    >
      <label className="switch-row">
        {S.remind}
        <input type="checkbox" role="switch" {...{ switch: "" }} checked={f.on} onChange={(e) => setF({ ...f, on: e.target.checked })} />
      </label>
      <label className="field15">
        <span>{S.time}</span>
        <input type="time" value={f.time} disabled={!f.on} onChange={(e) => e.target.value && setF({ ...f, time: e.target.value })} />
      </label>
      <label className="field15">
        <span>{S.lead}</span>
        <select className="pick15" value={f.lead} disabled={!f.on} onChange={(e) => setF({ ...f, lead: Number(e.target.value) })}>
          {LEAD_DAYS.map((n) => <option key={n} value={n}>{S.days(n)}</option>)}
        </select>
      </label>
      <SaveBar dirty={dirty} pending={save.isPending} ok={save.isSuccess} err={save.isError} />
    </form>
  );
}

/**
 * Ngôn ngữ & khu vực. Một ngôn ngữ chung của app (mẹ chốt 03/11): đổi thì lưu `lang` vào tài
 * khoản rồi tải lại trang với giao diện mới. Tiền tệ hiển thị chọn riêng (VND, USD), mẹ chốt 03/11.
 */
export function SettingsLanguage() {
  const me = useMeData();
  const save = useSaveSettings();
  const tz = me?.settings.timezone || "Asia/Ho_Chi_Minh";
  const zones = TIMEZONES.includes(tz) ? TIMEZONES : [tz, ...TIMEZONES];
  const country: Country = me?.settings.country ?? ZONE_COUNTRY[tz] ?? "VN";
  return (
    <Sub title={S.language}>
      <div className="box15 set15__form">
        <label className="field15">
          <span>{S.lang}</span>
          <select
            className="pick15"
            value={uiLang}
            disabled={save.isPending}
            onChange={(e) => {
              const lang = e.target.value as "vi" | "en";
              const go = () => switchUiLang(lang);
              save.mutate({ lang }, { onSuccess: go, onError: go });
            }}
          >
            <option value="vi">{LANG_NAME.vi}</option>
            <option value="en">{LANG_NAME.en}</option>
          </select>
        </label>
        <label className="field15">
          <span>{S.currency}</span>
          <select
            className="pick15"
            value={displayCurrency()}
            disabled={!me || save.isPending}
            onChange={(e) => {
              const currency = e.target.value as DisplayCurrency;
              const go = () => switchCurrency(currency);
              save.mutate({ currency }, { onSuccess: go, onError: go });
            }}
          >
            <option value="VND">{S.currencyName.VND}</option>
            <option value="USD">{S.currencyName.USD}</option>
          </select>
        </label>
        <label className="field15">
          <span>{S.country}</span>
          <select
            className="pick15"
            value={country}
            disabled={!me || save.isPending}
            onChange={(e) => save.mutate({ country: e.target.value as Country })}
          >
            {(COUNTRIES.includes(country) ? COUNTRIES : [...COUNTRIES, country]).map((c) => (
              <option key={c} value={c}>{S.countryName[c]}</option>
            ))}
          </select>
        </label>
        <label className="field15">
          <span>{S.tz}</span>
          <select className="pick15" value={tz} disabled={!me || save.isPending} onChange={(e) => save.mutate({ timezone: e.target.value })}>
            {zones.map((z) => <option key={z} value={z}>{z.replace(/_/g, " ")}</option>)}
          </select>
        </label>
        {save.isError && <p className="error" role="alert">{t.settings.saveFailed}</p>}
        {save.isSuccess && <p className="hint" role="status">{S.saved}</p>}
      </div>
      <p className="hint hint15"><Info size={16} aria-hidden /> {S.langNote}</p>
    </Sub>
  );
}

/** Thành viên: thẻ nhà có mã mời, danh sách người nhà, mời thành viên, rời nhà, vào nhà bằng mã. */
export function SettingsMembers() {
  const me = useMeData();
  const h = useHousehold();
  const act = useHouseholdActions();
  const [copied, setCopied] = useState(false);
  const [code, setCode] = useState("");
  const [confirmRenew, setConfirmRenew] = useState(false);
  const { relationOf } = useRelations();
  const data = h.data;
  if (!data) return <Sub title={S.members}><p className="hint">{t.common.loading}</p></Sub>;
  const isOwner = data.my_role === "owner";
  const invite = act.invite.data ?? data.invite;
  const owner = data.members.find((m) => m.role === "owner");
  const others = data.members.filter((m) => m.uid !== me?.uid);
  return (
    <Sub title={S.members}>
      <section className="box15 house15" aria-label={S.houseOf(owner?.display_name ?? "")}>
        <span className="house15__icon" aria-hidden>🏠</span>
        <span className="house15__text">
          <strong>{S.houseOf(owner?.display_name || data.name)}</strong>
          <small>{S.membersSub(data.members.length)}</small>
          {invite ? (
            <>
              <span className="house15__code">{S.houseCode(invite.code)}</span>
              <small>{S.codeFixed}</small>
            </>
          ) : (
            isOwner && <small>{S.noCode}</small>
          )}
        </span>
        {invite && (
          <button
            type="button"
            className="btn btn--quiet house15__copy"
            onClick={() => {
              navigator.clipboard?.writeText(invite.code).catch(() => undefined);
              setCopied(true);
            }}
          >
            <Copy size={18} aria-hidden /> {copied ? S.copied : S.copy}
          </button>
        )}
      </section>

      <ul className="box15 set15__list">
        {data.members.map((m) => {
          const rel = relationOf(m.uid);
          const role = m.role === "owner" ? S.owner : S.member;
          return (
            <li key={m.uid}>
              <Link to={`/settings/members/${encodeURIComponent(m.uid)}`} className="set15__row">
                <span className="set15__avatar" aria-hidden>{initial(m.display_name || "?")}</span>
                <span className="set15__text">
                  <span>{m.display_name || m1.assign.someone}{m.uid === me?.uid && ` (${S.you})`}</span>
                  <small>{rel ? `${S.relations[rel]} · ${role}` : role}</small>
                </span>
                <ChevronRight size={20} aria-hidden />
              </Link>
            </li>
          );
        })}
      </ul>

      {isOwner && !invite && (
        <button type="button" className="btn btn--primary btn--block btn--lg" aria-disabled={act.invite.isPending || undefined} onClick={() => !act.invite.isPending && act.invite.mutate()}>
          <Plus size={22} aria-hidden /> {S.invite}
        </button>
      )}
      {isOwner && invite && (
        <>
          <button
            type="button"
            className={`btn btn--quiet btn--block${confirmRenew ? " pd15__del--confirm" : ""}`}
            aria-disabled={act.invite.isPending || undefined}
            onClick={() => {
              if (act.invite.isPending) return;
              if (!confirmRenew) return setConfirmRenew(true);
              act.invite.mutate(undefined, { onSettled: () => { setConfirmRenew(false); setCopied(false); } });
            }}
          >
            {confirmRenew ? S.renewConfirm : S.renew}
          </button>
          {confirmRenew && <p className="hint" role="status">{S.renewAsk}</p>}
        </>
      )}
      {act.invite.isError && <p className="error" role="alert">{errText(act.invite.error)}</p>}

      <section className="box15 set15__note">
        <ShieldCheck size={22} aria-hidden />
        <span>
          <strong>{S.privacyTitle}</strong>
          <small>{S.privacyBody}</small>
        </span>
      </section>

      <ul className="box15 set15__list">
        <MandateRow />
      </ul>

      {others.length === 0 && (
        <form
          className="box15 set15__form"
          onSubmit={(e) => {
            e.preventDefault();
            if (code.trim().length !== 8 || act.join.isPending) return;
            act.join.mutate(code.trim().toUpperCase(), { onSuccess: () => setCode("") });
          }}
        >
          <label className="field15">
            <span>{S.join}</span>
            <input value={code} maxLength={8} autoCapitalize="characters" placeholder={m1.household.joinPlaceholder} onChange={(e) => setCode(e.target.value)} />
          </label>
          {act.join.isError && <p className="error" role="alert">{errText(act.join.error)}</p>}
          <button type="submit" className="btn btn--quiet btn--block" aria-disabled={code.trim().length !== 8 || act.join.isPending || undefined}>{S.joinCta}</button>
        </form>
      )}

      {data.id && data.members.length > 1 && (!isOwner || others.length === 0) && (
        <button type="button" className="btn btn--block btn--lg set15__leave" onClick={() => act.leave.mutate()}>{S.leave}</button>
      )}
      {act.leave.isError && <p className="error" role="alert">{errText(act.leave.error)}</p>}
    </Sub>
  );
}

/** Một người nhà: tên, Vai trong nhà (tạm lưu trên máy), nút xóa khỏi nhà cho chủ nhà. */
export function SettingsMember() {
  const { uid = "" } = useParams();
  const me = useMeData();
  const h = useHousehold();
  const act = useHouseholdActions();
  const nav = useNavigate();
  const { relationOf, setRelation, error: relErr } = useRelations();
  const rel = relationOf(uid);
  const m = h.data?.members.find((x) => x.uid === uid);
  if (!h.data) return <Sub title={S.members}><p className="hint">{t.common.loading}</p></Sub>;
  if (!m) return <Sub title={S.members}><p className="hint">{t.common.genericError}</p></Sub>;
  const canRemove = h.data.my_role === "owner" && m.uid !== me?.uid;
  return (
    <Sub title={m.display_name || S.members}>
      <section className="box15 set15__form">
        <span className="set15__avatar set15__avatar--lg" aria-hidden>{initial(m.display_name || "?")}</span>
        <p className="set15__who">
          <strong>{m.display_name}{m.uid === me?.uid && ` (${S.you})`}</strong>
          <small>{m.role === "owner" ? S.owner : S.member}</small>
        </p>
        <label className="field15">
          <span>{S.relation}</span>
          <select className="pick15" value={rel} onChange={(e) => setRelation(uid, e.target.value)}>
            <option value="">{S.relationNone}</option>
            {RELATIONS.map((r) => <option key={r} value={r}>{S.relations[r]}</option>)}
          </select>
        </label>
        <p className="hint">{S.relationNote}</p>
        {relErr && <p className="error" role="alert">{t.common.genericError}</p>}
      </section>
      {canRemove && (
        <button
          type="button"
          className="btn btn--block btn--lg set15__leave"
          aria-disabled={act.removeMember.isPending || undefined}
          onClick={() => !act.removeMember.isPending && act.removeMember.mutate(m.uid, { onSuccess: () => nav("/settings/members", { replace: true }) })}
        >
          {S.remove(m.display_name)}
        </button>
      )}
      {act.removeMember.isError && <p className="error" role="alert">{errText(act.removeMember.error)}</p>}
    </Sub>
  );
}

/** Quyền riêng tư & bảo mật: tạm là trang chữ (chốt 31/10). */
export function SettingsPrivacy() {
  const hidden = useHideAmounts();
  return (
    <Sub title={S.privacy}>
      {/* B2 (theo Redesign): ẩn số tiền, lưu trên máy. */}
      <div className="box15 set15__form">
        <label className="switch-row">
          <span className="set15__text">
            <span>{S.hideAmounts}</span>
            <small>{S.hideAmountsSub}</small>
          </span>
          <input type="checkbox" role="switch" {...{ switch: "" }} checked={hidden} onChange={(e) => setHideAmounts(e.target.checked)} />
        </label>
      </div>
      <section className="box15 set15__page">
        {S.privacyPage.map((p) => <p key={p}>{p}</p>)}
      </section>
      <DeleteAccount />
    </Sub>
  );
}

/**
 * B10: xóa tài khoản. Bấm Xóa vĩnh viễn thì gọi DELETE /me; máy chủ chỉ nhận khi vừa đăng
 * nhập lại trong 5 phút, không thì trả reauth_required và màn này mời đăng nhập lại (Google
 * chuyển trang, hoặc link email quay về đúng trang này), rồi mẹ bấm Xóa vĩnh viễn lần nữa.
 */
function DeleteAccount() {
  const D = S.del;
  const auth = useAuth();
  const me = useMeData();
  const h = useHousehold();
  const del = useDeleteAccount();
  const nav = useNavigate();
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const [needReauth, setNeedReauth] = useState(false);
  const [reauthBusy, setReauthBusy] = useState(false);

  // Quay về từ Google hoặc từ link email sau khi đăng nhập lại.
  useEffect(() => {
    auth.completeReauthIfPresent().then((r) => {
      if (r === "ok") {
        setOpen(true);
        setNeedReauth(false);
        setNote(D.reauthed);
      } else if (r === "expired") {
        setOpen(true);
        setNeedReauth(true);
        setNote(D.linkExpired);
      }
    });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const others = (h.data?.members.length ?? 1) - 1;
  const isOwner = !h.data || h.data.owner_uid === me?.uid;
  const reauthErr = del.error instanceof ApiError && del.error.code === "reauth_required";

  const remove = () => {
    if (del.isPending) return;
    setNote(null);
    del.mutate(undefined, {
      onSuccess: async () => {
        await wipeDeviceData();
        await auth.signOut();
        nav("/", { replace: true });
      },
      onError: (e) => {
        if (e instanceof ApiError && e.code === "reauth_required") setNeedReauth(true);
      },
    });
  };
  const reauth = async () => {
    if (reauthBusy) return;
    setReauthBusy(true);
    try {
      const r = await auth.startReauth("/settings/privacy");
      if (r === "done") {
        setNeedReauth(false);
        setNote(D.reauthed);
      } else if (r === "link_sent") {
        setNote(D.linkSent(auth.session?.email ?? ""));
      }
    } catch {
      setNote(t.common.genericError);
    } finally {
      setReauthBusy(false);
    }
  };

  return (
    <section className="box15 set15__page" aria-labelledby="del-h">
      <h2 id="del-h">{D.title}</h2>
      <p>{D.body}</p>
      {h.data && others > 0 && <p>{isOwner ? D.ownerNote(others) : D.memberNote}</p>}
      {!open ? (
        <button type="button" className="btn btn--block btn--danger" onClick={() => setOpen(true)}>{D.start}</button>
      ) : (
        <>
          {needReauth && !note && <p className="hint" role="status">{D.needReauth}</p>}
          {note && <p className="hint" role="status">{note}</p>}
          {del.isError && !reauthErr && <p className="error" role="alert">{errText(del.error)}</p>}
          {needReauth ? (
            <button type="button" className="btn btn--primary btn--block" aria-disabled={reauthBusy || undefined} onClick={reauth}>{D.reauth}</button>
          ) : (
            <button type="button" className="btn btn--block btn--danger" aria-disabled={del.isPending || undefined} onClick={remove}>
              {del.isPending ? D.deleting : D.confirm}
            </button>
          )}
          <button
            type="button"
            className="btn btn--block"
            onClick={() => {
              setOpen(false);
              setNeedReauth(false);
              setNote(null);
              del.reset();
            }}
          >
            {D.cancel}
          </button>
        </>
      )}
    </section>
  );
}

const MANDATE_CATS: MandateCategory[] = ["grocery", "household"];

/** Dòng Ủy quyền mua online ở Cài đặt: hiện đang bật hay tắt, bấm vào để sửa. */
function MandateRow() {
  const q = useMandate();
  return <Row to="/settings/mandate" Icon={Wallet} label={S.mandate} chip={q.data ? (q.data.enabled ? S.on : S.off) : undefined} />;
}

export function SettingsMandate() {
  return (
    <Sub title={S.mandate}>
      <MandateSection />
    </Sub>
  );
}

/** Household Mandate (Mimi 2.0 mục 13): phạm vi Mimi được tự thanh toán online. Chỉ chủ nhà sửa được. */
export function MandateSection() {
  const q = useMandate();
  return (
    <section className="plan-sec" aria-labelledby="mandate-h">
      <h2 id="mandate-h">{t.mandate.title}</h2>
      <p className="hint">{t.mandate.intro}</p>
      {q.data && <MandateForm initial={q.data} />}
      <Link to="/orders" className="link-row">
        <span>{t.settings.ordersLink}</span>
        <ChevronRight size={20} aria-hidden />
      </Link>
    </section>
  );
}

function MandateForm({ initial }: { initial: Mandate }) {
  const [m, setM] = useState<Mandate>(initial);
  const save = useSaveMandate();
  const toggleCat = (c: MandateCategory, on: boolean) =>
    setM({ ...m, categories: on ? [...m.categories.filter((x) => x !== c), c] : m.categories.filter((x) => x !== c) });
  const num = (v: string, max: number) => Math.min(max, Math.max(0, Math.round(Number(v) || 0)));

  return (
    <form
      className="plan-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (!save.isPending) save.mutate(m);
      }}
    >
      <label className="switch-row">
        {t.mandate.enabled}
        <input type="checkbox" role="switch" {...{ switch: "" }} checked={m.enabled} onChange={(e) => setM({ ...m, enabled: e.target.checked })} />
      </label>
      <fieldset className="plan-form">
        <legend>{t.mandate.categories}</legend>
        {MANDATE_CATS.map((c) => (
          <label key={c} className="switch-row">
            {t.mandate.categoryNames[c]}
            <input type="checkbox" checked={m.categories.includes(c)} onChange={(e) => toggleCat(c, e.target.checked)} />
          </label>
        ))}
      </fieldset>
      <label>
        {t.mandate.tolerance}
        <input type="number" inputMode="numeric" min={0} max={50} value={m.price_tolerance_pct} onChange={(e) => setM({ ...m, price_tolerance_pct: num(e.target.value, 50) })} />
      </label>
      <label>
        {t.mandate.maxOrder}
        <input type="number" inputMode="numeric" min={0} step={10000} value={m.max_order_vnd} onChange={(e) => setM({ ...m, max_order_vnd: num(e.target.value, 100_000_000) })} />
      </label>
      {save.isError && <p className="error" role="alert">{t.settings.saveFailed}</p>}
      <button type="submit" className="btn btn--quiet btn--block" aria-disabled={save.isPending || undefined}>
        {save.isPending ? t.confirm.saving : save.isSuccess ? t.mandate.saved : t.mandate.save}
      </button>
    </form>
  );
}

/** Spec 1.5 mục 8: "Đang chờ đồng bộ (N)", từng việc kèm giờ, nút Thử lại. Ẩn khi không còn gì chờ. */
function SyncBox() {
  const q = useQueue();
  const [busy, setBusy] = useState(false);
  if (q.length === 0) return null;
  const hm = (ms: number) => new Date(ms).toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" });
  return (
    <section className="box15 sync15" aria-label={m15.sync.title(q.length)}>
      <h2>{m15.sync.title(q.length)}</h2>
      <ul>
        {q.map((w) => (
          <li key={w.key}><span>{w.label ?? m15.sync.generic}</span><small>{hm(w.queuedAt)}</small></li>
        ))}
      </ul>
      <button type="button" className="btn btn--outline" aria-disabled={busy || undefined} onClick={async () => { setBusy(true); try { await flushQueue(); } finally { setBusy(false); } }}>
        {m15.sync.retry}
      </button>
      <p className="hint">{m15.sync.note}</p>
    </section>
  );
}
