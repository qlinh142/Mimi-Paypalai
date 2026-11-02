import { t } from "../i18n";
import { Link } from "react-router-dom";
import { MainFrame, TopBar } from "../components/AppShell";
import { useAuth } from "../lib/auth";
import { wipeDeviceData } from "../lib/localData";

/** Màn khung: đã có route và vị trí trong điều hướng, nội dung làm theo spec ở bước sau. */
export function Stub({ title, section, sub = false }: { title: string; section: string; sub?: boolean }) {
  return (
    <MainFrame>
      <TopBar title={title} variant={sub ? "sub" : "main"} />
      <div className="page stub empty">
        <h2>{title}</h2>
        <p>{t.stub.body(section)}</p>
        <Link className="btn btn--quiet" to="/">{t.common.backHome}</Link>
      </div>
    </MainFrame>
  );
}

/** Tài khoản: tạm có nút Đăng xuất (spec mục 6, đề xuất đặt ở đây). */
export function Account() {
  const auth = useAuth();
  return (
    <MainFrame>
      <TopBar title={t.account.title} variant="sub" />
      <div className="page empty">
        <p>{auth.session?.email ?? ""}</p>
        <button
          className="btn btn--quiet"
          onClick={async () => {
            // Đăng xuất phải dọn cả cache dữ liệu và hàng đợi ghi trên máy.
            await wipeDeviceData();
            await auth.signOut();
          }}
        >
          {t.account.signOut}
        </button>
      </div>
    </MainFrame>
  );
}

export function TodayMeal() {
  return (
    <main className="frame">
      <TopBar title={t.meal.title} variant="sub" />
      <div className="page empty">
        <h2>{t.meal.heading}</h2>
        <p>{t.meal.body}</p>
        <Link className="btn btn--quiet" to="/">{t.common.backHome}</Link>
      </div>
    </main>
  );
}
