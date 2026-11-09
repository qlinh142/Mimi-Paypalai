/**
 * Nạp Braintree Drop-in UI một lần cho lựa chọn "Trả bằng thẻ". Số thẻ do Drop-in gửi
 * thẳng cho Braintree; app chỉ nhận lại nonce dùng một lần và gửi cho server Mimi.
 * Server chạy BRAINTREE_MODE=fake thì trả FAKE_CLIENT_TOKEN, app dùng nút giả lập.
 */
export const FAKE_CLIENT_TOKEN = "fake-client-token";
/** Nonce thẻ hợp lệ của Braintree Sandbox; server giả lập chỉ nhận nonce này. */
export const FAKE_NONCE = "fake-valid-nonce";

const DROPIN_VERSION = "1.47.0";

/** Kết quả Drop-in; liabilityShift* chỉ có khi đã chạy 3D Secure. */
export type DropinPayload = {
  nonce: string;
  liabilityShifted?: boolean;
  liabilityShiftPossible?: boolean;
};
export type DropinInstance = {
  requestPaymentMethod: (opts?: { threeDSecure?: { amount: string } }) => Promise<DropinPayload>;
  teardown: () => Promise<void>;
};
type DropinNamespace = {
  create: (opts: {
    authorization: string;
    container: HTMLElement;
    locale?: string;
    card?: Record<string, unknown>;
    threeDSecure?: boolean;
  }) => Promise<DropinInstance>;
};

/**
 * 3D Secure: ngân hàng phát hành xác minh chủ thẻ (mã OTP hoặc app ngân hàng) trước khi
 * Mimi thu tiền. Nếu ngân hàng hỗ trợ 3DS mà xác minh không thành (liability không
 * chuyển sang ngân hàng), app không thu và mời dùng thẻ khác, theo khuyến nghị Braintree.
 */
export function threeDSecureRejected(p: DropinPayload): boolean {
  return p.liabilityShiftPossible === true && p.liabilityShifted === false;
}

let loading: Promise<DropinNamespace> | null = null;

export function loadDropin(): Promise<DropinNamespace> {
  if (loading) return loading;
  loading = new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = `https://js.braintreegateway.com/web/dropin/${DROPIN_VERSION}/js/dropin.min.js`;
    s.async = true;
    s.onload = () => {
      const bt = (window as unknown as { braintree?: { dropin?: DropinNamespace } }).braintree;
      if (bt?.dropin) resolve(bt.dropin);
      else reject(new Error("Braintree Drop-in did not load"));
    };
    s.onerror = () => {
      loading = null;
      reject(new Error("Braintree Drop-in failed to load"));
    };
    document.head.appendChild(s);
  });
  return loading;
}
