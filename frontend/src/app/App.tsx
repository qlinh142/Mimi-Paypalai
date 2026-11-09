import { lazy, Suspense, useEffect, useRef } from "react";
import { BrowserRouter, HashRouter, MemoryRouter, Navigate, Route, Routes } from "react-router-dom";

// Bản prototype một file (vite.proto.config.ts) dùng HashRouter vì không có máy chủ trả mọi đường dẫn về index.html.
// Mở trong khung xem bị cô lập (origin "null", ví dụ trình xem file của app Claude) thì HashRouter
// không dựng được URL: dùng MemoryRouter, đường dẫn chỉ nằm trong bộ nhớ.
const Router =
  import.meta.env.VITE_ROUTER === "hash" ? (typeof window !== "undefined" && window.origin === "null" ? MemoryRouter : HashRouter) : BrowserRouter;
import { PersistQueryClientProvider } from "@tanstack/react-query-persist-client";
import { configureApi, flushQueue } from "../api/client";
import { setQueueOwner } from "../lib/offlineQueue";
import { claimDevice } from "../lib/localData";
import { UndoProvider, UpdateBanner } from "../components/AppShell";
import { AuthProvider, useAuth } from "../lib/auth";
import { queryClient, queryPersister } from "./queryClient";
import { Welcome } from "../screens/Welcome";
import { Home } from "../screens/Home";
import { LangSync } from "./LangSync";
import { MoneySync } from "./MoneySync";

// Màn ít vào ngay được tách bundle riêng để lần mở đầu nhẹ.
const Items = lazy(() => import("../screens/Items").then((m) => ({ default: m.Items })));
const ItemDetail = lazy(() => import("../screens/ItemDetail").then((m) => ({ default: m.ItemDetail })));
const Scan = lazy(() => import("../screens/Scan").then((m) => ({ default: m.Scan })));
const PlanList = lazy(() => import("../screens/PlanList").then((m) => ({ default: m.PlanList })));
const PlanDetail = lazy(() => import("../screens/PlanDetail").then((m) => ({ default: m.PlanDetail })));
const PlanAdd = lazy(() => import("../screens/PlanFlow22").then((m) => ({ default: m.PlanAdd })));
const PlanCompare = lazy(() => import("../screens/PlanFlow22").then((m) => ({ default: m.PlanCompare })));
const PlanConfirm = lazy(() => import("../screens/PlanFlow22").then((m) => ({ default: m.PlanConfirm })));
const PlanDone = lazy(() => import("../screens/PlanFlow22").then((m) => ({ default: m.PlanDone })));
const ReceiptDetailScreen = lazy(() => import("../screens/PurchaseHistory").then((m) => ({ default: m.ReceiptDetailScreen })));
const RecurringDetail = lazy(() => import("../screens/RecurringDetail").then((m) => ({ default: m.RecurringDetail })));
const ShoppingRun = lazy(() => import("../screens/ShoppingRun").then((m) => ({ default: m.ShoppingRun })));
const Spending = lazy(() => import("../screens/Spending").then((m) => ({ default: m.Spending })));
const Analytics = lazy(() => import("../analytics/AnalyticsScreen").then((m) => ({ default: m.Analytics })));
const Settings = lazy(() => import("../screens/Settings").then((m) => ({ default: m.Settings })));
const lazySet = (k: "SettingsProfile" | "SettingsNotify" | "SettingsLanguage" | "SettingsMembers" | "SettingsMember" | "SettingsPrivacy" | "SettingsMandate") =>
  lazy(() => import("../screens/Settings").then((m) => ({ default: m[k] })));
const SettingsProfile = lazySet("SettingsProfile");
const SettingsNotify = lazySet("SettingsNotify");
const SettingsLanguage = lazySet("SettingsLanguage");
const SettingsMembers = lazySet("SettingsMembers");
const SettingsMember = lazySet("SettingsMember");
const SettingsPrivacy = lazySet("SettingsPrivacy");
const SettingsMandate = lazySet("SettingsMandate");
const Notifications = lazy(() => import("../screens/Notifications").then((m) => ({ default: m.Notifications })));
const Account = lazy(() => import("../screens/Stub").then((m) => ({ default: m.Account })));
const OnlineBuy = lazy(() => import("../screens/Online").then((m) => ({ default: m.OnlineBuy })));
const Orders = lazy(() => import("../screens/Online").then((m) => ({ default: m.Orders })));
const OrderDetail = lazy(() => import("../screens/Online").then((m) => ({ default: m.OrderDetail })));
const TodayMeal = lazy(() => import("../screens/Stub").then((m) => ({ default: m.TodayMeal })));

function Gate() {
  const auth = useAuth();
  const wasSignedOut = useRef<boolean | null>(null);

  useEffect(() => {
    configureApi({
      getToken: auth.idToken,
      onUnauthorized: () => auth.signOut(), // 401: về màn đăng nhập (spec mục 1)
    });
  }, [auth]);

  useEffect(() => {
    setQueueOwner(auth.session?.uid ?? null);
    if (!auth.session) return;
    void claimDevice(auth.session.uid);
    flushQueue();
    const onOnline = () => flushQueue();
    window.addEventListener("online", onOnline);
    return () => window.removeEventListener("online", onOnline);
  }, [auth.session]);

  if (!auth.ready) return null;
  if (wasSignedOut.current === null) wasSignedOut.current = !auth.session;
  if (!auth.session) {
    wasSignedOut.current = true;
    return (
      <Routes>
        <Route path="*" element={<Welcome />} />
      </Routes>
    );
  }

  return (
    <Suspense fallback={null}>
      <LangSync />
      <MoneySync>
      <Routes>
        <Route path="/" element={<Home entering={wasSignedOut.current === true} />} />
        <Route path="/items" element={<Items />} />
        <Route path="/items/recurring/:id" element={<RecurringDetail />} />
        <Route path="/items/:key" element={<ItemDetail />} />
        <Route path="/scan" element={<Scan />} />
        <Route path="/spending" element={<Spending />} />
        {/* Link cũ: Lịch sử đơn hàng nay ở tab thứ ba của Kế hoạch (anh chốt 09/11). */}
        <Route path="/spending/history" element={<Navigate to="/plan?tab=history" replace />} />
        <Route path="/spending/analytics" element={<Analytics />} />
        <Route path="/spending/receipt/:id" element={<ReceiptDetailScreen />} />
        <Route path="/plan" element={<PlanList />} />
        <Route path="/plan/run/:id" element={<ShoppingRun />} />
        <Route path="/plan/:id/add" element={<PlanAdd />} />
        <Route path="/plan/:id/compare" element={<PlanCompare />} />
        <Route path="/plan/:id/confirm" element={<PlanConfirm />} />
        <Route path="/plan/:id/done" element={<PlanDone />} />
        <Route path="/plan/:id" element={<PlanDetail />} />
        <Route path="/plan/online" element={<OnlineBuy />} />
        <Route path="/orders" element={<Orders />} />
        <Route path="/orders/:id" element={<OrderDetail />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="/settings/account" element={<Account />} />
        <Route path="/settings/profile" element={<SettingsProfile />} />
        <Route path="/settings/notify" element={<SettingsNotify />} />
        <Route path="/settings/language" element={<SettingsLanguage />} />
        <Route path="/settings/members" element={<SettingsMembers />} />
        <Route path="/settings/members/:uid" element={<SettingsMember />} />
        <Route path="/settings/privacy" element={<SettingsPrivacy />} />
        <Route path="/settings/mandate" element={<SettingsMandate />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/today" element={<TodayMeal />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      </MoneySync>
    </Suspense>
  );
}

export function App() {
  return (
    <PersistQueryClientProvider client={queryClient} persistOptions={{ persister: queryPersister, maxAge: 7 * 24 * 60 * 60 * 1000 }}>
      <AuthProvider>
        <Router>
          <UndoProvider>
            <Gate />
            <UpdateBanner />
          </UndoProvider>
        </Router>
      </AuthProvider>
    </PersistQueryClientProvider>
  );
}
