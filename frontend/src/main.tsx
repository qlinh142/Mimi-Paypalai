import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/screens.css";
import "./styles/m15.css";
import "./styles/s22.css";
import "./styles/v2.css";
import { App } from "./app/App";
import { startPwaUpdates } from "./app/pwaUpdate";

startPwaUpdates();

// Giao diện mới (Mimi UI Redesign Spec): bật mặc định, tắt bằng VITE_UI_V2=0 khi build.
// Thử nhanh trên máy: thêm ?uiv2=0 hoặc ?uiv2=1 vào địa chỉ, lựa chọn được nhớ trên máy đó.
(() => {
  let on = import.meta.env.VITE_UI_V2 !== "0";
  try {
    const q = new URLSearchParams(location.search).get("uiv2");
    if (q === "0" || q === "1") localStorage.setItem("mimi.uiv2", q);
    const saved = localStorage.getItem("mimi.uiv2");
    if (saved === "0" || saved === "1") on = saved === "1";
  } catch {
    /* trình duyệt chặn bộ nhớ: dùng giá trị khi build */
  }
  if (on) document.documentElement.dataset.ui = "v2";
})();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
