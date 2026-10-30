import { t, uiLang } from "../i18n";
import { useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { useMandate, useMe, useSaveMandate, useSaveSettings } from "../api/hooks";
import type { Mandate, MandateCategory, Me, SettingsPatch } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { HouseholdPanel } from "../components/Household";

const LEAD_DAYS = [1, 2, 3, 5, 7, 10, 14, 21, 30];
const TIMEZONES = [
  "Asia/Ho_Chi_Minh", "Asia/Bangkok", "Asia/Singapore", "Asia/Tokyo", "Asia/Seoul",
  "Australia/Sydney", "Europe/London", "Europe/Berlin", "America/New_York", "America/Los_Angeles",
];

type Form = {
  display_name: string;
  remind_enabled: boolean;
  digest_time: string;
  lead_days: number;
  lang: "vi" | "en";
  timezone: string;
};

function fromMe(me: Me): Form {
  const s = me.settings;
  return {
    display_name: me.display_name ?? "",
    remind_enabled: s.remind_enabled ?? true,
    digest_time: s.digest_time ?? "08:00",
    lead_days: s.lead_days ?? 7,
    lang: s.lang ?? "vi",
    timezone: s.timezone || "Asia/Ho_Chi_Minh",
  };
}

/** Chỉ gửi các trường đã đổi: PUT /settings giữ nguyên trường vắng mặt. */
function diff(a: Form, b: Form): SettingsPatch {
  const p: SettingsPatch = {};
  if (a.display_name.trim() !== b.display_name.trim()) p.display_name = b.display_name.trim();
  if (a.remind_enabled !== b.remind_enabled) p.remind_enabled = b.remind_enabled;
  if (a.digest_time !== b.digest_time) p.digest_time = b.digest_time;
  if (a.lead_days !== b.lead_days) p.lead_days = b.lead_days;
  if (a.lang !== b.lang) p.lang = b.lang;
  if (a.timezone !== b.timezone) p.timezone = b.timezone;
  return p;
}

/** Cài đặt (spec mục 6). Phần chia sẻ với người nhà chưa có API nên chỉ ghi chú. */
export function Settings() {
  const me = useMe();
  return (
    <MainFrame>
      <TopBar title={t.settings.title} />
      <div className="page">
        {me.isPending && <p className="hint">{t.common.loading}</p>}
        {me.isError && !me.data && (
          <div className="empty">
            <p>{t.common.genericError}</p>
            <button className="btn btn--quiet" onClick={() => me.refetch()}>{t.common.retry}</button>
          </div>
        )}
        {me.data && <SettingsForm me={me.data} />}

        <HouseholdPanel />

        <MandateSection />

        <section className="plan-sec">
          <h2>{t.settings.account}</h2>
          <Link to="/settings/account" className="link-row">
            <span>{t.settings.accountLink}</span>
            <ChevronRight size={20} aria-hidden />
          </Link>
        </section>
      </div>
    </MainFrame>
  );
}

function SettingsForm({ me }: { me: Me }) {
  const [saved, setSaved] = useState<Form>(() => fromMe(me));
  const [f, setF] = useState<Form>(saved);
  const save = useSaveSettings();
  const patch = diff(saved, f);
  const dirty = Object.keys(patch).length > 0;
  const zones = TIMEZONES.includes(f.timezone) ? TIMEZONES : [f.timezone, ...TIMEZONES];

  return (
    <form
      className="plan-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (!dirty || save.isPending) return;
        save.mutate(patch, { onSuccess: () => setSaved(f) });
      }}
    >
      <section className="plan-sec">
        <h2>{t.settings.profile}</h2>
        <label>
          {t.settings.displayName}
          <input value={f.display_name} maxLength={40} autoComplete="given-name" onChange={(e) => setF({ ...f, display_name: e.target.value })} />
        </label>
      </section>

      <section className="plan-sec plan-form">
        <h2>{t.settings.reminders}</h2>
        <label className="switch-row">
          {t.settings.remindEnabled}
          <input
            type="checkbox"
            role="switch"
            {...{ switch: "" }}
            checked={f.remind_enabled}
            onChange={(e) => setF({ ...f, remind_enabled: e.target.checked })}
          />
        </label>
        <div className="plan-form__row">
          <label>
            {t.settings.digestTime}
            <input type="time" value={f.digest_time} disabled={!f.remind_enabled} onChange={(e) => e.target.value && setF({ ...f, digest_time: e.target.value })} />
          </label>
          <label>
            {t.settings.leadDays}
            <select value={f.lead_days} onChange={(e) => setF({ ...f, lead_days: Number(e.target.value) })}>
              {LEAD_DAYS.map((n) => (
                <option key={n} value={n}>{t.settings.leadDaysOption(n)}</option>
              ))}
            </select>
          </label>
        </div>
      </section>

      <section className="plan-sec plan-form">
        <h2>{t.settings.receipts}</h2>
        <label>
          {t.settings.receiptLang}
          <select value={f.lang} onChange={(e) => setF({ ...f, lang: e.target.value as Form["lang"] })}>
            <option value="vi">{t.settings.receiptLangVi}</option>
            <option value="en">{t.settings.receiptLangEn}</option>
          </select>
        </label>
      </section>

      <section className="plan-sec plan-form">
        <h2>{t.settings.region}</h2>
        <label>
          {t.settings.timezone}
          <select value={f.timezone} onChange={(e) => setF({ ...f, timezone: e.target.value })}>
            {zones.map((z) => (
              <option key={z} value={z}>{z.replace(/_/g, " ")}</option>
            ))}
          </select>
        </label>
        <label>
          {t.settings.appLanguage}
          {/* Ngôn ngữ giao diện lưu trên máy (không gửi lên API); đổi xong tải lại trang. */}
          <select value={uiLang} onChange={(e) => location.assign(`/settings?ui=${e.target.value}`)}>
            <option value="en">English</option>
            <option value="vi">Tiếng Việt</option>
          </select>
        </label>
      </section>

      {save.isError && <p className="error" role="alert">{t.settings.saveFailed}</p>}
      <button type="submit" className="btn btn--primary btn--block btn--lg" aria-disabled={!dirty || save.isPending || undefined}>
        {save.isPending ? t.confirm.saving : save.isSuccess && !dirty ? t.settings.saved : t.settings.save}
      </button>
    </form>
  );
}

const MANDATE_CATS: MandateCategory[] = ["grocery", "household"];

/** Household Mandate (Mimi 2.0 mục 13): phạm vi Mimi được tự thanh toán online. Chỉ chủ nhà sửa được. */
function MandateSection() {
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
      <div className="plan-form__row">
        <label>
          {t.mandate.maxOrder}
          <input type="number" inputMode="numeric" min={0} step={10000} value={m.max_order_vnd} onChange={(e) => setM({ ...m, max_order_vnd: num(e.target.value, 100_000_000) })} />
        </label>
        <label>
          {t.mandate.maxDay}
          <input type="number" inputMode="numeric" min={0} step={10000} value={m.max_day_vnd} onChange={(e) => setM({ ...m, max_day_vnd: num(e.target.value, 100_000_000) })} />
        </label>
      </div>
      {save.isError && <p className="error" role="alert">{t.settings.saveFailed}</p>}
      <button type="submit" className="btn btn--quiet btn--block" aria-disabled={save.isPending || undefined}>
        {save.isPending ? t.confirm.saving : save.isSuccess ? t.mandate.saved : t.mandate.save}
      </button>
    </form>
  );
}
