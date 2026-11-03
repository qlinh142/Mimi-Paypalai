import { switchUiLang, t, uiLang } from "../i18n";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useAuth } from "../lib/auth";
import { API_MODE } from "../lib/firebase";
import { MimiMark } from "../components/MimiMark";

type Phase =
  | { kind: "form"; notice?: string }
  | { kind: "sending" }
  | { kind: "sent"; email: string }
  | { kind: "ask_email" }; // mở link ở máy khác

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;

/** Trang chủ khi chưa đăng nhập: giới thiệu và đăng nhập ngay trên màn (spec mục 1). */
export function Welcome() {
  const auth = useAuth();
  const [phase, setPhase] = useState<Phase>({ kind: "form" });
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const emailRef = useRef<HTMLInputElement>(null);
  const ids = { email: useId(), name: useId(), err: useId(), help: useId() };

  // Người dùng vừa bấm link trong email.
  useEffect(() => {
    auth.completeLinkIfPresent().then((r) => {
      if (r === "need_email") setPhase({ kind: "ask_email" });
      if (r === "expired") setPhase({ kind: "form", notice: t.welcome.linkExpired });
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function validate(v: string) {
    return EMAIL_RE.test(v.trim()) ? null : t.welcome.invalidEmail;
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (phase.kind === "sending") return; // bấm hai lần chỉ gửi một thư
    const err = validate(email);
    setEmailError(err);
    if (err) {
      emailRef.current?.focus();
      return;
    }
    if (phase.kind === "ask_email") {
      const r = await auth.completeLinkIfPresent(email.trim());
      if (r === "expired") setPhase({ kind: "form", notice: t.welcome.linkExpired });
      return;
    }
    setPhase({ kind: "sending" });
    try {
      await auth.sendLink(email.trim(), name);
      setPhase({ kind: "sent", email: email.trim() });
    } catch {
      setPhase({ kind: "form", notice: navigator.onLine ? t.welcome.sendFailed : t.welcome.sendFailedOffline });
    }
  }

  const askOnly = phase.kind === "ask_email";
  const sent = phase.kind === "sent" ? t.welcome.sentBody(phase.email) : null;

  return (
    <main className="frame">
      <div className="welcome">
        <div className="welcome__brand">
          <MimiMark size={132} animated />
          <h1 className="welcome__title">
            {t.welcome.title}
            <span>{t.welcome.subtitle}</span>
          </h1>
        </div>

        {phase.kind === "sent" ? (
          <section className="sent" aria-live="polite">
            <h2>{t.welcome.sentTitle}</h2>
            <p>
              {sent!.before}
              <strong>{sent!.email}</strong>
              {sent!.after}
            </p>
            <button className="btn btn--quiet" type="button" onClick={() => setPhase({ kind: "form" })}>
              {t.welcome.resend}
            </button>
            {API_MODE === "mock" && (
              <a className="btn btn--primary" href="?mockLink=1">
                {t.welcome.openMockLink}
              </a>
            )}
          </section>
        ) : (
          <form className="welcome__card" onSubmit={onSubmit} noValidate>
            {askOnly && <p className="hint">{t.welcome.otherDevice}</p>}
            {phase.kind === "form" && phase.notice && (
              <p className="error" role="alert">{phase.notice}</p>
            )}
            <div className="field">
              <label htmlFor={ids.email}>{t.welcome.email}</label>
              <input
                ref={emailRef}
                id={ids.email}
                className="input"
                name="email"
                type="email"
                inputMode="email"
                autoComplete="email"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
                enterKeyHint={askOnly ? "go" : "next"}
                placeholder={t.welcome.emailPlaceholder}
                required
                value={email}
                aria-invalid={emailError ? true : undefined}
                aria-describedby={emailError ? ids.err : undefined}
                onChange={(e) => {
                  setEmail(e.target.value);
                  if (emailError) setEmailError(null);
                }}
                onBlur={() => email && setEmailError(validate(email))}
              />
              {emailError && (
                <span id={ids.err} className="error" role="alert">{emailError}</span>
              )}
            </div>
            {!askOnly && (
              <div className="field">
                <label htmlFor={ids.name}>{t.welcome.name}</label>
                <input
                  id={ids.name}
                  className="input"
                  name="display_name"
                  autoComplete="nickname"
                  enterKeyHint="send"
                  maxLength={40}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  aria-describedby={ids.help}
                />
                <span id={ids.help} className="hint">{t.welcome.nameHint}</span>
              </div>
            )}
            <button
              className="btn btn--primary btn--block btn--lg"
              type="submit"
              aria-disabled={phase.kind === "sending" || undefined}
            >
              {phase.kind === "sending" ? t.welcome.sending : askOnly ? t.welcome.signIn : t.welcome.submit}
            </button>

            {!askOnly && (
              <>
                <p className="welcome__or">{t.welcome.or}</p>
                <button className="btn btn--quiet btn--block btn--google" type="button" onClick={() => auth.signInGoogle()}>
                  <GoogleG />
                  {t.welcome.google}
                </button>
              </>
            )}
          </form>
        )}

        <footer className="welcome__foot">
          <span>{t.welcome.footer}</span>
          <span className="lang" role="group" aria-label={t.welcome.langGroup}>
            <button type="button" aria-pressed={uiLang === "vi"} onClick={() => uiLang !== "vi" && switchUiLang("vi")}>VI</button>
            <button type="button" aria-pressed={uiLang === "en"} onClick={() => uiLang !== "en" && switchUiLang("en")}>EN</button>
          </span>
        </footer>
      </div>
    </main>
  );
}

/** Logo "G" của Google cho nút đăng nhập, theo hướng dẫn thương hiệu Sign in with Google. */
function GoogleG() {
  return (
    <svg width="20" height="20" viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
      <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
      <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" />
      <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
    </svg>
  );
}
