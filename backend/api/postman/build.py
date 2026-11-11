#!/usr/bin/env python3
"""Sinh bộ Postman cho API Mimi từ backend/api/openapi.yaml.

    python3 backend/api/postman/build.py

Ra 3 file cạnh script này:
  - mimi.postman_collection.json: mọi endpoint (gom theo tag) + thư mục "Kịch bản thanh toán sandbox"
    chạy lần lượt PayPal (tạo đơn, duyệt, capture, giao, kiểm hàng, hoàn tiền) và Braintree (thẻ).
  - mimi-live.postman_environment.json: gọi mimi-1-5.web.app (dán token lấy từ trình duyệt).
  - mimi-local.postman_environment.json: gọi backend chạy ở máy (localhost:8080, PAYPAL_MODE=fake).

Sửa openapi.yaml thì chạy lại script; không sửa tay file JSON.
"""
import json
import pathlib
import uuid

import yaml

HERE = pathlib.Path(__file__).resolve().parent
SPEC = yaml.safe_load((HERE.parent / "openapi.yaml").read_text(encoding="utf-8"))
SCHEMAS = SPEC["components"]["schemas"]
PARAMS = SPEC["components"].get("parameters", {})

# id cố định để Postman nhận là cùng một bộ khi nhập lại (ghi đè thay vì tạo bản sao).
COLLECTION_ID = "6b1d5d8e-3f0e-4f43-9a52-6d1f6f0b2a11"


def resolve(node):
    while isinstance(node, dict) and "$ref" in node:
        name = node["$ref"].split("/")[-1]
        node = SCHEMAS.get(name) or PARAMS.get(name) or {}
    return node


def example(schema, depth=0):
    """Thân request mẫu: chỉ các trường bắt buộc, giá trị giữ chỗ dễ sửa."""
    s = resolve(schema)
    if depth > 4 or not isinstance(s, dict):
        return None
    if "example" in s:
        return s["example"]
    if "allOf" in s:
        out = {}
        for part in s["allOf"]:
            v = example(part, depth + 1)
            if isinstance(v, dict):
                out.update(v)
        return out
    if "oneOf" in s or "anyOf" in s:
        for part in s.get("oneOf") or s.get("anyOf"):
            if resolve(part).get("type") != "null":
                return example(part, depth + 1)
        return None
    t = s.get("type")
    if isinstance(t, list):
        t = next((x for x in t if x != "null"), None)
    if "enum" in s:
        return s["enum"][0]
    if t == "object" or "properties" in s:
        props = s.get("properties", {})
        req = s.get("required") or []
        return {k: example(props[k], depth + 1) for k in req if k in props}
    if t == "array":
        return [example(s.get("items", {}), depth + 1)]
    if t == "integer":
        return 0
    if t == "number":
        return 0
    if t == "boolean":
        return False
    if s.get("format") == "date":
        return "2026-11-11"
    return ""


def url(path, query=None):
    raw = "{{baseUrl}}" + path.replace("{", ":").replace("}", "")
    parts = [p for p in path.strip("/").split("/")]
    variables = [{"key": p[1:-1], "value": "{{" + {"id": "orderId", "key": "itemKey"}.get(p[1:-1], p[1:-1]) + "}}"}
                 for p in parts if p.startswith("{")]
    u = {"raw": raw, "host": ["{{baseUrl}}"], "path": [p.replace("{", ":").replace("}", "") for p in parts]}
    if variables:
        u["variable"] = variables
    if query:
        u["query"] = query
        u["raw"] += "?" + "&".join(f"{q['key']}={q['value']}" for q in query)
    return u


def headers(method, json_body):
    h = [{"key": "X-Firebase-AppCheck", "value": "{{appCheckToken}}", "description": "Bỏ trống khi máy chủ chưa bắt buộc App Check"}]
    if method in ("POST", "PUT", "PATCH", "DELETE"):
        h.append({"key": "Idempotency-Key", "value": "{{$guid}}", "description": "Mỗi lần bấm Send một key mới"})
    if json_body:
        h.append({"key": "Content-Type", "value": "application/json"})
    return h


def request(method, path, op):
    body = None
    rb = resolve(op.get("requestBody") or {})
    content = rb.get("content", {})
    if "application/json" in content:
        body = {"mode": "raw", "raw": json.dumps(example(content["application/json"].get("schema", {})), ensure_ascii=False, indent=2),
                "options": {"raw": {"language": "json"}}}
    elif "multipart/form-data" in content:
        props = resolve(content["multipart/form-data"].get("schema", {})).get("properties", {})
        body = {"mode": "formdata", "formdata": [{"key": k, "type": "file", "src": []} for k in props]}
    elif "application/x-www-form-urlencoded" in content:
        props = resolve(content["application/x-www-form-urlencoded"].get("schema", {})).get("properties", {})
        body = {"mode": "urlencoded", "urlencoded": [{"key": k, "value": ""} for k in props]}
    query = []
    for p in op.get("parameters", []):
        p = resolve(p)
        if p.get("in") == "query":
            query.append({"key": p["name"], "value": str(resolve(p.get("schema", {})).get("default", "")), "disabled": not p.get("required")})
    req = {"method": method, "header": headers(method, body and body["mode"] == "raw"), "url": url(path, query or None)}
    if body:
        req["body"] = body
    desc = (op.get("description") or "").strip()
    if op.get("x-status") == "planned":
        desc = ("Chưa làm: máy chủ trả 501.\n\n" + desc).strip()
    if desc:
        req["description"] = desc
    if op.get("security") == []:
        req["auth"] = {"type": "noauth"}
    return req


def item(name, method, path, op=None, body=None, tests=None, prerequest=None, description=None, query=None):
    op = op or SPEC["paths"][path][method.lower()]
    req = request(method, path, op)
    if query is not None:
        req["url"] = url(path, query)
    if body is not None:
        req["body"] = {"mode": "raw", "raw": json.dumps(body, ensure_ascii=False, indent=2) if not isinstance(body, str) else body,
                       "options": {"raw": {"language": "json"}}}
        req["header"] = headers(method, True)
    if description:
        req["description"] = description
    out = {"name": name, "request": req}
    ev = []
    if prerequest:
        ev.append({"listen": "prerequest", "script": {"type": "text/javascript", "exec": prerequest.strip().split("\n")}})
    if tests:
        ev.append({"listen": "test", "script": {"type": "text/javascript", "exec": tests.strip().split("\n")}})
    if ev:
        out["event"] = ev
    return out


def reference_folders():
    tags = [t["name"] for t in SPEC.get("tags", [])]
    tag_desc = {t["name"]: t.get("description", "") for t in SPEC.get("tags", [])}
    folders = {t: [] for t in tags}
    for path, ops in SPEC["paths"].items():
        for method, op in ops.items():
            if method not in ("get", "post", "put", "patch", "delete"):
                continue
            tag = (op.get("tags") or ["system"])[0]
            name = f"{method.upper()} {path}"
            if op.get("summary"):
                name += f": {op['summary']}"
            folders.setdefault(tag, []).append(item(name[:200], method.upper(), path, op))
    return [{"name": f"API: {t}", "description": tag_desc.get(t, ""), "item": folders[t]} for t in folders if folders[t]]


OK = 'pm.test("Thành công (2xx)", () => pm.expect(pm.response.code).to.be.within(200, 299));'

PAYPAL = [
    item("1. Kiểm tra đăng nhập", "GET", "/me", tests=OK),
    item("2. Lấy món đầu tiên trong Kế hoạch", "GET", "/plan", tests=OK + """
const items = (pm.response.json().items || []);
pm.test("Kế hoạch có ít nhất 1 món", () => pm.expect(items.length).to.be.above(0));
if (items.length) { pm.collectionVariables.set("itemKey", items[0].key); console.log("Món dùng để thử:", items[0].name, items[0].key); }
"""),
    item("3. Dựng giỏ và xem Mandate", "POST", "/orders/basket", body={"keys": ["{{itemKey}}"]}, tests=OK),
    item("4. Tạo đơn PayPal", "POST", "/orders", body={"keys": ["{{itemKey}}"], "approve": True, "provider": "paypal"}, tests=OK + """
const o = pm.response.json();
pm.collectionVariables.set("orderId", o.id);
pm.collectionVariables.set("approveUrl", o.approve_url || "");
pm.test("Đơn chờ thanh toán", () => pm.expect(o.state).to.eql("awaiting_payment"));
console.log("Mở link này, đăng nhập tài khoản người mua sandbox và bấm duyệt, rồi chạy bước 5:", o.approve_url);
""", description="Sau bước này, mở `approveUrl` (xem Console hoặc biến collection) bằng trình duyệt, đăng nhập tài khoản người mua PayPal sandbox và duyệt. Chế độ PAYPAL_MODE=fake ở máy thì bỏ qua bước duyệt."),
    item("5. Capture sau khi duyệt", "POST", "/orders/{id}/capture", tests=OK + """
pm.test("Đơn đã trả", () => pm.expect(pm.response.json().state).to.eql("paid"));
"""),
    item("6. Cửa hàng sandbox: đặt hàng", "POST", "/orders/{id}/advance", tests=OK),
    item("7. Cửa hàng sandbox: đang giao", "POST", "/orders/{id}/advance", tests=OK),
    item("8. Cửa hàng sandbox: đã giao", "POST", "/orders/{id}/advance", tests=OK + """
pm.test("Đã giao", () => pm.expect(pm.response.json().state).to.eql("delivered"));
"""),
    item("9. Kiểm hàng: thiếu cả món", "POST", "/orders/{id}/verify", body='{\n  "actual": { "{{itemKey}}": 0 }\n}', tests=OK + """
const o = pm.response.json();
pm.collectionVariables.set("refundVnd", String(o.suggested_refund_vnd || 0));
pm.test("Có số tiền gợi ý hoàn", () => pm.expect(o.suggested_refund_vnd).to.be.above(0));
"""),
    item("10. Hoàn tiền phần thiếu qua PayPal", "POST", "/orders/{id}/refund", body='{\n  "reason": "missing",\n  "amount_vnd": {{refundVnd}},\n  "note": "Thiếu hàng (kịch bản Postman)"\n}', tests=OK + """
const o = pm.response.json();
pm.test("Đã ghi hoàn tiền", () => pm.expect(o.refunded_vnd).to.be.above(0));
"""),
    item("11. Xem lại đơn", "GET", "/orders/{id}", tests=OK),
]

BRAINTREE = [
    item("1. Tạo đơn trả bằng thẻ", "POST", "/orders", body={"keys": ["{{itemKey}}"], "approve": True, "provider": "braintree"}, tests=OK + """
pm.collectionVariables.set("orderId", pm.response.json().id);
""", description="Chạy bước 2 của kịch bản PayPal trước để có `itemKey`."),
    item("2. Lấy client token Braintree", "POST", "/orders/card-token", tests=OK),
    item("3. Thu tiền bằng nonce thử", "POST", "/orders/{id}/card", body={"nonce": "fake-valid-nonce"}, tests=OK + """
pm.test("Đơn đã trả", () => pm.expect(pm.response.json().state).to.eql("paid"));
""", description="`fake-valid-nonce` là thẻ thử của Braintree sandbox (Visa hợp lệ). Thẻ thật không bao giờ qua máy chủ Mimi."),
]


# Môi trường local có authEmulator: tự đăng nhập Auth emulator lấy idToken (hết hạn sau 1 giờ thì lấy lại).
EMULATOR_LOGIN = """
const emu = pm.environment.get("authEmulator");
const exp = Number(pm.environment.get("idTokenExp") || 0);
if (emu && (!pm.environment.get("idToken") || Date.now() > exp)) {
  pm.sendRequest({
    url: emu + "/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake",
    method: "POST",
    header: { "Content-Type": "application/json" },
    body: { mode: "raw", raw: JSON.stringify({ email: "postman-" + Date.now() + "@example.com", password: "postman-test", returnSecureToken: true }) },
  }, (err, res) => {
    if (err || res.code !== 200) { console.error("Auth emulator:", err || res.text()); return; }
    pm.environment.set("idToken", res.json().idToken);
    pm.environment.set("idTokenExp", String(Date.now() + 50 * 60 * 1000));
  });
}
"""


def collection():
    return {
        "info": {
            "_postman_id": COLLECTION_ID,
            "name": "Mimi API",
            "description": (
                "Sinh từ backend/api/openapi.yaml bằng backend/api/postman/build.py. Xem README.md cùng thư mục.\n\n"
                + SPEC["info"].get("description", "")
            ),
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{idToken}}", "type": "string"}]},
        "variable": [{"key": k, "value": ""} for k in ("itemKey", "orderId", "approveUrl", "refundVnd")],
        "event": [{"listen": "prerequest", "script": {"type": "text/javascript", "exec": EMULATOR_LOGIN.strip().split("\n")}}],
        "item": [
            {"name": "Kịch bản thanh toán sandbox", "description": "Chạy theo thứ tự bằng Collection Runner hoặc bấm Send từng bước.",
             "item": [{"name": "PayPal: mua, giao, kiểm hàng, hoàn tiền", "item": PAYPAL},
                      {"name": "Braintree: trả bằng thẻ", "item": BRAINTREE}]},
            *reference_folders(),
        ],
    }


def environment(name, base, note, extra=()):
    return {
        "id": str(uuid.uuid5(uuid.NAMESPACE_URL, "mimi-postman-" + name)),
        "name": name,
        "values": [
            {"key": "baseUrl", "value": base, "type": "default", "enabled": True},
            {"key": "idToken", "value": "", "type": "secret", "enabled": True},
            {"key": "appCheckToken", "value": "", "type": "secret", "enabled": True},
            *extra,
        ],
        "_postman_variable_scope": "environment",
        "_note": note,
    }


def main():
    def write(name, data):
        (HERE / name).write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    write("mimi.postman_collection.json", collection())
    write("mimi-live.postman_environment.json",
          environment("Mimi live (mimi-1-5)", "https://mimi-1-5.web.app/api", "Dán idToken và appCheckToken lấy từ trình duyệt, xem README."))
    write("mimi-local.postman_environment.json",
          environment("Mimi local", "http://localhost:8080/api", "Backend chạy ở máy với PAYPAL_MODE=fake, BRAINTREE_MODE=fake, APP_CHECK_REQUIRED=false.",
                      [{"key": "authEmulator", "value": "http://localhost:9099", "type": "default", "enabled": True}]))


if __name__ == "__main__":
    main()
