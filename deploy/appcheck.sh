#!/usr/bin/env bash
# Bật App Check (reCAPTCHA Enterprise) cho Mimi, làm hai bước để không khóa nhầm người dùng.
#
#   bash deploy/appcheck.sh mimi-1-5          # bước 1: tạo khóa, đăng ký App Check, build lại frontend
#   bash deploy/appcheck.sh mimi-1-5 on       # bước 2: máy chủ bắt buộc App Check (sau khi app vẫn chạy bình thường)
#   bash deploy/appcheck.sh mimi-1-5 off      # quay lại: máy chủ không bắt buộc nữa
set -euo pipefail

PROJECT="${1:?Cách dùng: bash deploy/appcheck.sh <project-id> [on|off]}"
MODE="${2:-setup}"
REGION="asia-southeast1"
KEY_NAME="mimi-web"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
say() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
gcloud config set project "$PROJECT" >/dev/null

case "$MODE" in
  on|off)
    v=$([ "$MODE" = on ] && echo true || echo false)
    say "mimi-api: APP_CHECK_REQUIRED=$v"
    gcloud run services update mimi-api --region "$REGION" --update-env-vars "APP_CHECK_REQUIRED=$v" --quiet
    echo "Kiểm tra: curl -s https://$PROJECT.web.app/api/ready"
    [ "$MODE" = on ] && echo "Nếu app báo lỗi khi lưu hay đọc dữ liệu: bash deploy/appcheck.sh $PROJECT off"
    exit 0 ;;
  setup) ;;
  *) echo "Không hiểu '$MODE' (on | off, hoặc bỏ trống)"; exit 1 ;;
esac

say "1/4 Bật API reCAPTCHA Enterprise và App Check"
gcloud services enable recaptchaenterprise.googleapis.com firebaseappcheck.googleapis.com

say "2/4 Khóa reCAPTCHA cho tên miền app"
KEY="$(gcloud recaptcha keys list --filter="displayName=$KEY_NAME" --format='value(name)' | head -1)"
KEY="${KEY##*/}"
if [ -z "$KEY" ]; then
  KEY="$(gcloud recaptcha keys create --display-name="$KEY_NAME" --web \
    --domains="$PROJECT.web.app,$PROJECT.firebaseapp.com" --integration-type=score --format='value(name)')"
  KEY="${KEY##*/}"
fi
echo "  khóa: $KEY"

say "3/4 Đăng ký khóa với App Check của app web"
TOKEN="$(gcloud auth print-access-token)"
APP_ID="$(curl -fsS -H "Authorization: Bearer $TOKEN" "https://firebase.googleapis.com/v1beta1/projects/$PROJECT/webApps" |
  python3 -c 'import json,sys; a=json.load(sys.stdin).get("apps",[]); print(a[0]["appId"] if a else "")')"
[ -n "$APP_ID" ] || { echo "Chưa có app web trong Firebase"; exit 1; }
curl -fsS -X PATCH -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -H "X-Goog-User-Project: $PROJECT" \
  "https://firebaseappcheck.googleapis.com/v1/projects/$PROJECT/apps/$APP_ID/recaptchaEnterpriseConfig?updateMask=siteKey" \
  -d "{\"siteKey\":\"$KEY\"}" >/dev/null
echo "  đã đăng ký cho $APP_ID"

say "4/4 Build lại frontend có khóa và đưa lên Hosting"
ENVF="$ROOT/frontend/.env.production"
[ -f "$ENVF" ] || { echo "Chưa có $ENVF, chạy deploy/cloudshell-setup.sh trước"; exit 1; }
sed -i '/^VITE_RECAPTCHA_ENTERPRISE_KEY=/d' "$ENVF"
echo "VITE_RECAPTCHA_ENTERPRISE_KEY=$KEY" >> "$ENVF"
(cd "$ROOT/frontend" && npm run build)
(cd "$ROOT/frontend" && FIREBASE_HOSTING_UPLOAD_CONCURRENCY=5 firebase deploy --only hosting --project "$PROJECT" --non-interactive)

say "Xong bước 1. Máy chủ chưa bắt buộc App Check."
echo "Mở app, đăng nhập, thử vài màn. Ổn thì chạy: bash deploy/appcheck.sh $PROJECT on"
