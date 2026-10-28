/**
 * Nạp PayPal JS SDK một lần (Mimi 2.0 mục 15). Chỉ dùng khi có VITE_PAYPAL_CLIENT_ID
 * (Sandbox hoặc Live). Không có client ID thì màn Mua online dùng nút giả lập sandbox,
 * backend chạy PAYPAL_MODE=fake.
 */
export const PAYPAL_CLIENT_ID: string = import.meta.env.VITE_PAYPAL_CLIENT_ID ?? "";

/** Phần SDK mà Mimi dùng: nút PayPal với createOrder và onApprove. */
export type PayPalButtons = {
  render: (el: HTMLElement) => Promise<void>;
  close?: () => Promise<void>;
};
export type PayPalNamespace = {
  Buttons: (opts: {
    style?: Record<string, string>;
    createOrder: () => Promise<string>;
    onApprove: () => Promise<void>;
    onError?: (err: unknown) => void;
    onCancel?: () => void;
  }) => PayPalButtons;
};

let loading: Promise<PayPalNamespace> | null = null;

export function loadPayPal(): Promise<PayPalNamespace> {
  if (!PAYPAL_CLIENT_ID) return Promise.reject(new Error("PayPal client id missing"));
  if (loading) return loading;
  loading = new Promise((resolve, reject) => {
    const s = document.createElement("script");
    // PayPal không nhận VND: đơn tính bằng USD theo tỷ giá backend trả về.
    s.src = `https://www.paypal.com/sdk/js?client-id=${encodeURIComponent(PAYPAL_CLIENT_ID)}&currency=USD&intent=capture&components=buttons`;
    s.async = true;
    s.onload = () => {
      const pp = (window as unknown as { paypal?: PayPalNamespace }).paypal;
      if (pp) resolve(pp);
      else reject(new Error("PayPal SDK did not load"));
    };
    s.onerror = () => {
      loading = null;
      reject(new Error("PayPal SDK failed to load"));
    };
    document.head.appendChild(s);
  });
  return loading;
}
