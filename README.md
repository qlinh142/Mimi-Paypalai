# Mimi — the household shopping assistant

Mimi helps a family keep track of what they have at home, what is running out, what things cost
and how much they spend. Snap a receipt and Mimi reads it; plan the next shopping trip and Mimi
compares prices; pay online with PayPal and the order lands back in the family's spending book.

- **Live demo:** https://mimi-1-5.web.app/ (sandbox mode, no real money)
- **Version submitted:** 2.2
- **License:** MIT (see [LICENSE](LICENSE))
- **Team:** SMARTLIFE LAB

## Demo account for judges

To see Mimi with household data already in it (50 receipts from September to November 2026:
30 in-store receipts from US retailers and 20 online orders found through Channel3 and paid with
PayPal sandbox), open the live demo and tap **"Try the demo account"** on the sign-in screen.
No email or password to type. (If you need them: `judge@example.com` / `MimiDemo2026!`.)

The account is shared, so please don't change its settings. You can also sign up with your own
email or Google account to start from an empty household and scan your own receipts. Payments
run in PayPal / Braintree sandbox; the Braintree test card is `4111 1111 1111 1111`, any future
expiry, CVV `123`.

## How Mimi uses PayPal

All payments run on the **PayPal developer platform in Sandbox mode**.

| PayPal tool | What it does in Mimi |
|---|---|
| **PayPal JavaScript SDK (PayPal Buttons)** | The PayPal button on the checkout screen (`VITE_PAYPAL_CLIENT_ID`, sandbox app). |
| **PayPal REST API: Orders v2** (`/v2/checkout/orders`) | When the user approves a shopping basket, the backend creates a PayPal order and the user approves it on PayPal. After approval Mimi captures the order and writes it into the spending book as a new receipt. |
| **PayPal REST API: Payments v2** (`/v2/payments/captures/{id}/refund`) | If an item is missing on delivery, Mimi issues a full or partial refund and subtracts it from spending. |
| **Braintree (a PayPal service), Sandbox** | "Pay by card" next to the PayPal button: Drop-in UI with 3D Secure, card token and charge through the Braintree GraphQL API, refunds through the same gateway, and webhooks (settled, declined, disputes) update the order status. |

Code: `backend/internal/paypal`, `backend/internal/braintree`, `backend/internal/order`, `frontend/src/screens/Online.tsx`.

## How Mimi uses AI

Mimi uses **Google Gemini on Vertex AI** (the live demo runs `gemini-3.8-flash`, set through `GEMINI_MODEL`).

| AI feature | What it does |
|---|---|
| **Receipt reading** | Reads a photo of a paper receipt: store, date, each line item, quantity, unit price. Items are matched to the family's item list. |
| **Online price lookup** | Looks up current prices for up to 5 items at a time, reading trusted store pages first (URL context) and falling back to Google Search. Every price keeps its source link and date. |
| **Delivery check** | Counts what is visible in a photo of a delivered order and compares it to what was ordered, so only confirmed items go into the home inventory; missing items trigger a refund. |
| **"Ask Mimi"** | Answers questions about the family's own spending, using only data returned by the app's tools (no web search). |

Code: `backend/internal/gcp/gemini`, `backend/internal/assistant`.

## Sponsor tools

| Tool | What it does in Mimi |
|---|---|
| **AG Grid** (`ag-grid-react`) | The "Table" view of Order history: every receipt and online order in one sortable, filterable grid. |
| **AG Studio** and **AG Charts** | The "Analytics" screen: spending reports, charts and data tables built from the family's receipts. |
| **Channel3** | Product catalog and per-store prices in the U.S. market (Walmart, Target, Costco, CVS, Walgreens…), used to compare prices when planning a trip. Payment still goes through PayPal or Braintree. |

Code: `frontend/src/components/HistoryGrid.tsx` (AG Grid), `frontend/src/analytics/` (AG Studio), `backend/internal/channel3`.

## Other technology

- **Frontend:** React 19, TypeScript, Vite, installable PWA (works offline, queues changes).
- **Backend:** Go, one binary running as Cloud Run API, worker and scheduled jobs.
- **Google Cloud and Firebase:** Firestore, Firebase Auth (email link and Google sign-in), Firebase Hosting, App Check (reCAPTCHA Enterprise), Cloud Tasks, Cloud Scheduler, Secret Manager.

## Repository layout

```
frontend/   React app (see frontend/README.md)
backend/    Go API, worker and jobs (see backend/README.md)
deploy/     One-shot setup and deploy script for Google Cloud Shell
docs/       Specifications and design notes
```

## Run it locally

**Requirements:** Node.js 22+, Go 1.26+.

Frontend only, with built-in sample data (no backend or keys needed):

```bash
cd frontend
npm ci
cp .env.example .env # VITE_API_MODE=mock: sample data, no Firebase needed
npm run dev          # http://localhost:5173
```

In mock mode, enter any email, tap "Continue with email", then "Open link (demo)".

Backend (uses a fake PayPal and the Firestore emulator unless configured otherwise):

```bash
cd backend
make tidy
make test
make emulators       # in another terminal: Firestore and Auth emulators
make run-api
```

Main environment variables:

| Variable | Meaning |
|---|---|
| `PAYPAL_MODE` | `sandbox`, `live`, `fake` (no PayPal calls) or `off` |
| `PAYPAL_CLIENT_ID`, `PAYPAL_CLIENT_SECRET` | From developer.paypal.com → Apps & Credentials (sandbox app) |
| `BRAINTREE_MODE`, `BRAINTREE_PUBLIC_KEY` | `off`, `fake`, `sandbox` or `live`; private key from secret `braintree-private-key` |
| `GEMINI_MODEL`, `GEMINI_LOCATION` | Gemini model and Vertex AI region |

## Deploy to Google Cloud

From Google Cloud Shell, with the sandbox keys set:

```bash
PAYPAL_MODE=sandbox PAYPAL_CLIENT_ID=<sandbox client id> \
BRAINTREE_MODE=sandbox BRAINTREE_PUBLIC_KEY=<public key> \
bash deploy/cloudshell-setup.sh <project-id>
```

The script builds and deploys the backend to Cloud Run and the frontend to Firebase Hosting.
Secrets (PayPal client secret, Braintree private key, Channel3 API key) are read from Secret Manager;
the script tells you the exact command if one is missing.

## Tests

```bash
(cd backend && go test ./...)
(cd frontend && npm run build && npm run test:e2e)
```

CI (`.github/workflows/ci.yml`) runs backend tests, Firestore emulator tests, the frontend build and a secret scan on every push and pull request.

## Team

**SMARTLIFE LAB**

- **Lien Pham**: Founder, Product Lead
- **Quang Linh**: Engineer and Video
