import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  onIdTokenChanged,
  sendSignInLinkToEmail,
  isSignInWithEmailLink,
  signInWithEmailLink,
  signInWithRedirect,
  reauthenticateWithRedirect,
  reauthenticateWithCredential,
  getRedirectResult,
  EmailAuthProvider,
  GoogleAuthProvider,
  signOut as fbSignOut,
  type User,
} from "firebase/auth";
import { API_MODE, firebase } from "./firebase";

/**
 * Đăng nhập không mật khẩu (spec mục 1).
 * Email chỉ nằm ở Firebase Auth; tên Mimi gọi được gửi lên API sau khi đăng nhập
 * và lưu thành users/{uid}.display_name.
 */
type Session = { uid: string; email: string | null } | null;

type AuthCtx = {
  session: Session;
  ready: boolean;
  sendLink: (email: string, displayName: string) => Promise<void>;
  /** Trả về "need_email" khi link được mở ở máy khác (không có email lưu sẵn). */
  completeLinkIfPresent: (emailOverride?: string) => Promise<"none" | "ok" | "need_email" | "expired">;
  signInGoogle: () => Promise<void>;
  signOut: () => Promise<void>;
  idToken: () => Promise<string | null>;
  pendingDisplayName: () => string | null;
  /**
   * B10: đăng nhập lại để xác nhận thao tác nhạy cảm (xóa tài khoản). App không có mật khẩu
   * nên "nhập lại mật khẩu" là đăng nhập lại bằng Google (chuyển trang) hoặc link email.
   * returnPath là trang quay về sau khi đăng nhập lại.
   */
  startReauth: (returnPath: string) => Promise<"redirecting" | "link_sent" | "done">;
  /** Gọi khi mở trang returnPath: hoàn tất đăng nhập lại nếu vừa quay về từ Google hoặc link. */
  completeReauthIfPresent: () => Promise<"none" | "ok" | "expired">;
};

const EMAIL_KEY = "mimi.emailForSignIn";
const NAME_KEY = "mimi.pendingDisplayName";
const MOCK_KEY = "mimi.mock.session";
/** Chế độ mẫu: lúc "đăng nhập lại" gần nhất, để mock DELETE /me kiểm tra như máy chủ. */
export const MOCK_REAUTH_KEY = "mimi.mock.reauthAt";
const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (API_MODE === "mock") {
      const raw = localStorage.getItem(MOCK_KEY);
      setSession(raw ? JSON.parse(raw) : null);
      setReady(true);
      return;
    }
    const { auth } = firebase()!;
    return onIdTokenChanged(auth, (u: User | null) => {
      setSession(u ? { uid: u.uid, email: u.email } : null);
      setReady(true);
    });
  }, []);

  const value = useMemo<AuthCtx>(() => {
    const live = API_MODE === "live";
    return {
      session,
      ready,
      pendingDisplayName: () => localStorage.getItem(NAME_KEY),

      async sendLink(email, displayName) {
        localStorage.setItem(EMAIL_KEY, email);
        localStorage.setItem(NAME_KEY, displayName.trim());
        if (!live) {
          await new Promise((r) => setTimeout(r, 600));
          return;
        }
        await sendSignInLinkToEmail(firebase()!.auth, email, {
          url: `${location.origin}/`,
          handleCodeInApp: true,
        });
      },

      async completeLinkIfPresent(emailOverride) {
        if (!live) {
          if (!new URLSearchParams(location.search).has("mockLink")) return "none";
          const email = emailOverride ?? localStorage.getItem(EMAIL_KEY) ?? "ban@example.com";
          const s = { uid: "mock-uid", email };
          localStorage.setItem(MOCK_KEY, JSON.stringify(s));
          history.replaceState(null, "", location.pathname);
          setSession(s);
          return "ok";
        }
        const { auth } = firebase()!;
        if (!isSignInWithEmailLink(auth, location.href)) return "none";
        const email = emailOverride ?? localStorage.getItem(EMAIL_KEY);
        if (!email) return "need_email";
        try {
          await signInWithEmailLink(auth, email, location.href);
          localStorage.removeItem(EMAIL_KEY);
          history.replaceState(null, "", location.pathname);
          return "ok";
        } catch (e: unknown) {
          const code = (e as { code?: string }).code ?? "";
          if (code === "auth/invalid-action-code" || code === "auth/expired-action-code") {
            history.replaceState(null, "", location.pathname);
            return "expired";
          }
          throw e;
        }
      },

      async signInGoogle() {
        if (!live) return;
        // Redirect thay cho popup: popup hay bị chặn trong PWA iOS.
        // Cần authDomain = tên miền của app trên Firebase Hosting.
        await signInWithRedirect(firebase()!.auth, new GoogleAuthProvider());
      },

      async signOut() {
        if (!live) {
          localStorage.removeItem(MOCK_KEY);
          setSession(null);
          return;
        }
        await fbSignOut(firebase()!.auth);
      },

      async startReauth(returnPath) {
        if (!live) {
          sessionStorage.setItem(MOCK_REAUTH_KEY, String(Date.now()));
          return "done";
        }
        const u = firebase()!.auth.currentUser;
        if (!u) throw new Error("not signed in");
        if (u.providerData.some((p) => p.providerId === "google.com")) {
          await reauthenticateWithRedirect(u, new GoogleAuthProvider());
          return "redirecting";
        }
        if (!u.email) throw new Error("no email");
        localStorage.setItem(EMAIL_KEY, u.email);
        await sendSignInLinkToEmail(firebase()!.auth, u.email, { url: `${location.origin}${returnPath}`, handleCodeInApp: true });
        return "link_sent";
      },

      async completeReauthIfPresent() {
        if (!live) return "none";
        const { auth } = firebase()!;
        const u = auth.currentUser;
        if (!u) return "none";
        const link = location.href;
        if (isSignInWithEmailLink(auth, link)) {
          const email = localStorage.getItem(EMAIL_KEY) ?? u.email;
          history.replaceState(null, "", location.pathname);
          if (!email) return "expired";
          try {
            await reauthenticateWithCredential(u, EmailAuthProvider.credentialWithLink(email, link));
            localStorage.removeItem(EMAIL_KEY);
            return "ok";
          } catch {
            return "expired";
          }
        }
        // Quay về từ Google: kết quả đăng nhập lại nằm ở getRedirectResult.
        const r = await getRedirectResult(auth).catch(() => null);
        return r ? "ok" : "none";
      },

      async idToken() {
        if (!live) return session ? "mock-token" : null;
        const u = firebase()!.auth.currentUser;
        return u ? u.getIdToken() : null;
      },
    };
  }, [session, ready]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useAuth must be inside AuthProvider");
  return c;
}
