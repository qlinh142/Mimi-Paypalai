import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  onIdTokenChanged,
  sendSignInLinkToEmail,
  isSignInWithEmailLink,
  signInWithEmailLink,
  signInWithRedirect,
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
};

const EMAIL_KEY = "mimi.emailForSignIn";
const NAME_KEY = "mimi.pendingDisplayName";
const MOCK_KEY = "mimi.mock.session";
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
