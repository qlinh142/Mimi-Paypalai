#!/usr/bin/env bash
# Dựng và deploy Mimi lần đầu từ Cloud Shell (bản demo).
# Chạy lại nhiều lần được: bước nào đã có thì bỏ qua hoặc cập nhật.
#
#   bash deploy/cloudshell-setup.sh mimi-1-5
#
# Bản demo: MIMI_ENV=dev (thư chỉ ghi log, chưa gửi thật).
# App Check bật riêng bằng deploy/appcheck.sh; chạy lại script này vẫn giữ nguyên trạng thái App Check.
set -euo pipefail

PROJECT="${1:?Cách dùng: bash deploy/cloudshell-setup.sh <project-id>}"
REGION="asia-southeast1"
GEMINI_MODEL="${GEMINI_MODEL:-gemini-3.8-flash}" # dòng 2.5 đã bị Google tắt ngày 20/10/2026
PAYPAL_MODE="${PAYPAL_MODE:-off}" # bản test: PAYPAL_MODE=fake bash deploy/cloudshell-setup.sh mimi-1-5 (thanh toán giả, chưa cần khóa)
# PayPal Sandbox: PAYPAL_MODE=sandbox PAYPAL_CLIENT_ID=<Client ID> bash deploy/cloudshell-setup.sh mimi-1-5
# Secret cất trước trong Secret Manager tên paypal-client-secret (không bao giờ để trong repo):
#   gcloud secrets create paypal-client-secret --project mimi-1-5 --data-file=-
PAYPAL_CLIENT_ID="${PAYPAL_CLIENT_ID:-}"
# Braintree (nút "Trả bằng thẻ" cạnh PayPal). Thử giả lập: BRAINTREE_MODE=fake (chưa cần khóa).
# Sandbox: BRAINTREE_MODE=sandbox BRAINTREE_PUBLIC_KEY=<Public Key>; Private Key cất trước:
#   gcloud secrets create braintree-private-key --project mimi-1-5 --data-file=-
BRAINTREE_MODE="${BRAINTREE_MODE:-off}"
BRAINTREE_PUBLIC_KEY="${BRAINTREE_PUBLIC_KEY:-}"
BRAINTREE_MERCHANT_ACCOUNT_ID="${BRAINTREE_MERCHANT_ACCOUNT_ID:-}" # rỗng: tài khoản mặc định (phải là USD)
# AG Studio (màn Phân tích chi tiêu): khóa dùng thử 45 ngày. Để trống thì Studio vẫn chạy, chỉ hiện watermark.
#   AG_STUDIO_LICENSE=<khóa> bash deploy/cloudshell-setup.sh mimi-1-5
AG_STUDIO_LICENSE="${AG_STUDIO_LICENSE:-}"
GEMINI_THINKING="${GEMINI_THINKING:-LOW}" # LOW: đã thử đọc bill ổn. MINIMAL thử 02/11 thì AI từ chối mọi bill, đừng dùng
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

say() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }

gcloud config set project "$PROJECT" >/dev/null
SECRETS=()      # tên secret trong Secret Manager mà sa-api cần đọc
SECRET_ENVS=()  # ENV=secret:latest gắn vào Cloud Run
need_secret() { # $1 tên secret, $2 biến môi trường
  gcloud services enable secretmanager.googleapis.com --quiet
  gcloud secrets describe "$1" >/dev/null 2>&1 ||
    { echo "Chưa có secret $1. Chạy: gcloud secrets create $1 --project $PROJECT --data-file=-"; exit 1; }
  SECRETS+=("$1")
  SECRET_ENVS+=("$2=$1:latest")
}
if [ "$PAYPAL_MODE" = sandbox ] || [ "$PAYPAL_MODE" = live ]; then
  [ -n "$PAYPAL_CLIENT_ID" ] || { echo "Thiếu PAYPAL_CLIENT_ID cho PAYPAL_MODE=$PAYPAL_MODE"; exit 1; }
  need_secret paypal-client-secret PAYPAL_CLIENT_SECRET
fi
if [ "$BRAINTREE_MODE" = sandbox ] || [ "$BRAINTREE_MODE" = live ]; then
  [ -n "$BRAINTREE_PUBLIC_KEY" ] || { echo "Thiếu BRAINTREE_PUBLIC_KEY cho BRAINTREE_MODE=$BRAINTREE_MODE"; exit 1; }
  need_secret braintree-private-key BRAINTREE_PRIVATE_KEY
fi
# Channel3 (giá đồ dùng nhà, thuốc, đồ trẻ em ở Mỹ): có secret channel3-api-key thì bật, không có thì bỏ qua.
JOB_SECRET_FLAGS=()
if gcloud secrets describe channel3-api-key >/dev/null 2>&1; then
  need_secret channel3-api-key CHANNEL3_API_KEY
  JOB_SECRET_FLAGS=(--set-secrets "CHANNEL3_API_KEY=channel3-api-key:latest")
fi
SECRET_FLAGS=()
if [ ${#SECRET_ENVS[@]} -gt 0 ]; then
  SECRET_FLAGS=(--set-secrets "$(IFS=,; echo "${SECRET_ENVS[*]}")")
fi
NUM="$(gcloud projects describe "$PROJECT" --format='value(projectNumber)')"

say "1/7 Tài khoản dịch vụ và quyền"
for sa in sa-api sa-jobs sa-scheduler; do
  gcloud iam service-accounts describe "$sa@$PROJECT.iam.gserviceaccount.com" >/dev/null 2>&1 ||
    gcloud iam service-accounts create "$sa" --display-name "Mimi $sa"
done
bind() { gcloud projects add-iam-policy-binding "$PROJECT" --member "serviceAccount:$1@$PROJECT.iam.gserviceaccount.com" --role "$2" --condition=None >/dev/null; }
for r in roles/datastore.user roles/aiplatform.user roles/firebaseauth.admin roles/serviceusage.serviceUsageConsumer; do bind sa-api "$r"; done
for r in roles/datastore.user roles/firebaseauth.viewer roles/serviceusage.serviceUsageConsumer roles/aiplatform.user; do bind sa-jobs "$r"; done # aiplatform: price-sync tra giá bằng Gemini
bind sa-scheduler roles/run.invoker
if [ ${#JOB_SECRET_FLAGS[@]} -gt 0 ]; then # job price-sync cũng đọc key Channel3
  gcloud secrets add-iam-policy-binding channel3-api-key --member "serviceAccount:sa-jobs@$PROJECT.iam.gserviceaccount.com" \
    --role roles/secretmanager.secretAccessor --condition=None >/dev/null
fi
for sec in "${SECRETS[@]}"; do
  gcloud secrets add-iam-policy-binding "$sec" --member "serviceAccount:sa-api@$PROJECT.iam.gserviceaccount.com" \
    --role roles/secretmanager.secretAccessor --condition=None >/dev/null
done
# Build từ mã nguồn (Cloud Build) chạy bằng tài khoản Compute mặc định.
gcloud projects add-iam-policy-binding "$PROJECT" --member "serviceAccount:$NUM-compute@developer.gserviceaccount.com" \
  --role roles/run.builder --condition=None >/dev/null

say "2/7 Backend mimi-api lên Cloud Run (build mất khoảng 3 đến 5 phút)"
# Giữ nguyên App Check đang bật/tắt (deploy/appcheck.sh) khi deploy lại.
APP_CHECK="$(gcloud run services describe mimi-api --region "$REGION" --format=json 2>/dev/null |
  python3 -c 'import json,sys
try: e=json.load(sys.stdin)["spec"]["template"]["spec"]["containers"][0].get("env",[])
except Exception: e=[]
print(next((x.get("value","") for x in e if x.get("name")=="APP_CHECK_REQUIRED"),"false"))' || echo false)"
ENV_VARS="GOOGLE_CLOUD_PROJECT=$PROJECT,MIMI_ENV=dev,APP_CHECK_REQUIRED=$APP_CHECK,GEMINI_MODEL=$GEMINI_MODEL,GEMINI_THINKING=$GEMINI_THINKING,GEMINI_LOCATION=global,PAYPAL_MODE=$PAYPAL_MODE"
[ -n "$PAYPAL_CLIENT_ID" ] && ENV_VARS="$ENV_VARS,PAYPAL_CLIENT_ID=$PAYPAL_CLIENT_ID"
ENV_VARS="$ENV_VARS,BRAINTREE_MODE=$BRAINTREE_MODE"
[ -n "$BRAINTREE_PUBLIC_KEY" ] && ENV_VARS="$ENV_VARS,BRAINTREE_PUBLIC_KEY=$BRAINTREE_PUBLIC_KEY"
[ -n "$BRAINTREE_MERCHANT_ACCOUNT_ID" ] && ENV_VARS="$ENV_VARS,BRAINTREE_MERCHANT_ACCOUNT_ID=$BRAINTREE_MERCHANT_ACCOUNT_ID"
gcloud run deploy mimi-api --source "$ROOT/backend" --region "$REGION" \
  --service-account "sa-api@$PROJECT.iam.gserviceaccount.com" \
  --set-env-vars "$ENV_VARS" "${SECRET_FLAGS[@]}" --args serve-api \
  --allow-unauthenticated --min-instances 0 --max-instances 3 --memory 512Mi --timeout 60 --quiet
IMAGE="$(gcloud run services describe mimi-api --region "$REGION" --format='value(spec.template.spec.containers[0].image)')"

say "3/7 Các job (cùng image)"
job() { # tên, thời gian tối đa (mặc định 10m), cờ thêm
  local extra=("${@:3}")
  gcloud run jobs deploy "mimi-job-$1" --image "$IMAGE" --region "$REGION" "${extra[@]}" \
    --service-account "sa-jobs@$PROJECT.iam.gserviceaccount.com" \
    --set-env-vars "GOOGLE_CLOUD_PROJECT=$PROJECT,MIMI_ENV=dev,APP_CHECK_REQUIRED=false,GEMINI_MODEL=$GEMINI_MODEL,GEMINI_LOCATION=global" \
    --args "job,$1" --max-retries 1 --task-timeout "${2:-10m}" --quiet >/dev/null
  echo "  mimi-job-$1"
}
for j in fx-sync cleanup migrate seed-demo; do job "$j"; done
job price-sync 60m "${JOB_SECRET_FLAGS[@]}" # bảng giá chung: tối đa 200 món mỗi đêm, mỗi món vài giây

say "4/7 Lịch chạy job (giờ Việt Nam)"
sched() { # tên job, cron
  local uri="https://run.googleapis.com/v2/projects/$PROJECT/locations/$REGION/jobs/mimi-job-$1:run"
  local args=(--location "$REGION" --schedule "$2" --time-zone "Asia/Ho_Chi_Minh" --uri "$uri" --http-method POST
    --oauth-service-account-email "sa-scheduler@$PROJECT.iam.gserviceaccount.com")
  gcloud scheduler jobs describe "mimi-$1" --location "$REGION" >/dev/null 2>&1 &&
    gcloud scheduler jobs update http "mimi-$1" "${args[@]}" >/dev/null ||
    gcloud scheduler jobs create http "mimi-$1" "${args[@]}" >/dev/null
  echo "  mimi-$1: $2"
}
sched fx-sync "0 6 * * *"
sched cleanup "0 3 * * *"
sched price-sync "0 2 * * *"
echo "  chạy tỷ giá lần đầu"; gcloud run jobs execute mimi-job-fx-sync --region "$REGION" --wait --quiet >/dev/null || echo "  (chưa lấy được tỷ giá, 6h sáng mai lịch sẽ chạy lại)"

say "5/7 Cấu hình web Firebase cho frontend"
TOKEN="$(gcloud auth print-access-token)"
API="https://firebase.googleapis.com/v1beta1/projects/$PROJECT/webApps"
APP_ID="$(curl -fsS -H "Authorization: Bearer $TOKEN" "$API" | python3 -c 'import json,sys; a=json.load(sys.stdin).get("apps",[]); print(a[0]["appId"] if a else "")')"
[ -n "$APP_ID" ] || { echo "Chưa có app web trong Firebase (Project settings → Your apps)"; exit 1; }
curl -fsS -H "Authorization: Bearer $TOKEN" "$API/$APP_ID/config" | python3 -c '
import json,sys
c=json.load(sys.stdin)
print("VITE_API_MODE=live")
print("VITE_FIREBASE_API_KEY="+c["apiKey"])
# Google redirect cần authDomain trùng tên miền app (web.app), nếu không trình duyệt chặn cookie và quay về trắng.
print("VITE_FIREBASE_AUTH_DOMAIN="+c["projectId"]+".web.app")
print("VITE_FIREBASE_PROJECT_ID="+c["projectId"])
print("VITE_FIREBASE_APP_ID="+c["appId"])
print("VITE_USE_EMULATORS=false")' > "$ROOT/frontend/.env.production"
KEY="$(gcloud recaptcha keys list --filter="displayName=mimi-web" --format='value(name)' 2>/dev/null | head -1)"
[ -n "$KEY" ] && echo "VITE_RECAPTCHA_ENTERPRISE_KEY=${KEY##*/}" >> "$ROOT/frontend/.env.production"
[ -n "$PAYPAL_CLIENT_ID" ] && echo "VITE_PAYPAL_CLIENT_ID=$PAYPAL_CLIENT_ID" >> "$ROOT/frontend/.env.production"
[ -n "$AG_STUDIO_LICENSE" ] && echo "VITE_AG_STUDIO_LICENSE=$AG_STUDIO_LICENSE" >> "$ROOT/frontend/.env.production"
echo "  đã ghi frontend/.env.production"

say "6/7 Build frontend"
(cd "$ROOT/frontend" && npm ci --no-audit --no-fund && npm run build)

say "7/7 Đưa rules, index Firestore và frontend lên Firebase"
command -v firebase >/dev/null || npm install -g firebase-tools
firebase projects:list >/dev/null 2>&1 || firebase login --no-localhost
(cd "$ROOT/backend" && firebase deploy --only firestore:rules,firestore:indexes --project "$PROJECT" --non-interactive)
(cd "$ROOT/frontend" && FIREBASE_HOSTING_UPLOAD_CONCURRENCY=5 firebase deploy --only hosting --project "$PROJECT" --non-interactive)

say "Xong. Mở app: https://$PROJECT.web.app"
echo "Kiểm tra máy chủ: curl -s https://$PROJECT.web.app/api/ready"
echo "Lần đầu: thêm https://$PROJECT.web.app/__/auth/handler vào Authorized redirect URIs của OAuth client Web (APIs & Services → Credentials) để đăng nhập Google chạy."
