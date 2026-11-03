// Command mimi là binary duy nhất của backend Mimi, chạy một trong ba vai trò:
//
//	mimi serve-api      Cloud Run service mimi-api (ingress: all, sau Firebase Hosting)
//	mimi serve-worker   Cloud Run service mimi-worker (ingress: internal, nhận Cloud Tasks)
//	mimi job <name>     Cloud Run job do Cloud Scheduler gọi: digest | price-sync | fx-sync | cleanup
//
// main chỉ đọc cấu hình, nối phụ thuộc bằng constructor và chạy. Logic nằm trong internal/.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // image distroless/static không có tzdata hệ thống

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/account"
	"github.com/smartlife-lab/mimi/backend/internal/config"
	"github.com/smartlife-lab/mimi/backend/internal/demo"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
	"github.com/smartlife-lab/mimi/backend/internal/gcp/fbauth"
	"github.com/smartlife-lab/mimi/backend/internal/gcp/fsstore"
	"github.com/smartlife-lab/mimi/backend/internal/gcp/gemini"
	"github.com/smartlife-lab/mimi/backend/internal/gcp/tasks"
	"github.com/smartlife-lab/mimi/backend/internal/history"
	"github.com/smartlife-lab/mimi/backend/internal/home"
	"github.com/smartlife-lab/mimi/backend/internal/household"
	"github.com/smartlife-lab/mimi/backend/internal/httpapi"
	"github.com/smartlife-lab/mimi/backend/internal/inbox"
	"github.com/smartlife-lab/mimi/backend/internal/item"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/notify"
	"github.com/smartlife-lab/mimi/backend/internal/order"
	"github.com/smartlife-lab/mimi/backend/internal/paypal"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/platform/logx"
	"github.com/smartlife-lab/mimi/backend/internal/pricing"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	groupTaxonomy "github.com/smartlife-lab/mimi/backend/internal/taxonomy"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mimi:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mimi serve-api | serve-worker | job <name>")
	}
	// Cloud Run gửi SIGTERM trước khi dừng instance và cho khoảng 10 giây để dọn dẹp.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logx.New(os.Stdout, cfg.ProjectID, slog.LevelInfo)

	fs, err := firestore.NewClientWithDatabase(ctx, cfg.ProjectID, cfg.FirestoreDB)
	if err != nil {
		return fmt.Errorf("firestore: %w", err)
	}
	defer fs.Close()
	store := fsstore.New(fs)

	switch args[0] {
	case "serve-api":
		return serveAPI(ctx, cfg, log, store)
	case "serve-worker":
		return serveWorker(ctx, cfg, log, store)
	case "job":
		if len(args) < 2 {
			return errors.New("usage: mimi job <digest|migrate|fx-sync|cleanup|seed-demo|price-sync>")
		}
		return runJob(ctx, args[1], cfg, log, store)
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func serveAPI(ctx context.Context, cfg config.Config, log *slog.Logger, store *fsstore.Store) error {
	if err := cfg.RequireAPI(); err != nil {
		return err
	}
	auth, err := fbauth.New(ctx, cfg.ProjectID, cfg.AppCheckRequired)
	if err != nil {
		return err
	}
	taxonomy := groupTaxonomy.SpendingKeys()
	reader, err := gemini.New(ctx, cfg.ProjectID, cfg.GeminiLocation, cfg.GeminiModel, taxonomy, groupTaxonomy.Keys())
	if err == nil {
		reader.SetThinking(cfg.GeminiThinking)
	}
	if err != nil {
		return err
	}
	receipts, err := receipt.New(reader, store, store, store, store, log,
		receipt.WithDailyLimit(cfg.ReceiptDailyLimit),
		receipt.WithTaxonomy(taxonomy), receipt.WithPageCache(store))
	if err != nil {
		return err
	}
	var appCheck httpapi.AppCheckVerifier
	if cfg.AppCheckRequired {
		appCheck = auth
	}
	// B9: có cấu hình hàng đợi thì đẩy thông báo sự kiện cho worker gửi thư; chưa có thì
	// thông báo chỉ hiện trong app.
	if cfg.TasksQueue != "" {
		q, err := tasks.New(ctx, cfg.TasksQueue, cfg.WorkerURL, cfg.TasksSAEmail)
		if err != nil {
			return err
		}
		store.WithQueue(q)
	}
	items := item.New(store, nil)
	planSvc := plan.New(store, store, nil).WithPlans(store)
	payments, err := newPayments(cfg)
	if err != nil {
		return err
	}
	orders, err := order.New(store, store, order.NewSandboxCatalog(time.Now()), payments, reader, cfg.VNDPerUSD, log, nil)
	if err != nil {
		return err
	}
	orders.WithRateInfo(cfg.VNDRateSource, cfg.VNDRateAsOf)
	api, err := httpapi.New(httpapi.Deps{
		Log: log, Tokens: auth, AppCheck: appCheck, Principals: store, Profiles: store,
		Receipts: receipts, Items: items, Ready: store,
		Home:     home.New(store, items, planSvc, nil),
		Plan:     planSvc,
		Notes:    inbox.New(items, planSvc, nil).WithEvents(store),
		Spending: spending.New(store),
		Settings: user.NewSettingsService(store, nil),
		Orders:   orders,
		// Bộ spec Mimi 1.0: khoản định kỳ + dự báo (2.1), nhà + mua hộ (4.1), lịch sử giá (8.1).
		Recurring:  recurring.NewService(store, store, store, nil),
		Households: household.New(store, store, nil).WithReceipts(receipts),
		Pricing:    pricing.New(store, store, nil),
		Market:     store, // bảng giá chung, job price-sync ghi mỗi đêm
		// Nút "Cập nhật giá": tra lại giá món trong một kế hoạch, cùng nguồn công khai với job.
		PriceRefresh: market.NewRefresher(store, []market.Source{reader.NewPriceSearch(store, log)}, store, market.RefreshOptions{Log: log, FX: store}),
		History:      history.New(store),
		FX:           store, // tỷ giá từng ngày, app quy đổi sang tiền tệ hiển thị
		Accounts:     account.New(store, store, auth),
		SignIn:       auth,
	}, httpapi.WithTimeouts(cfg.ReadTimeout, cfg.DefaultTimeout), httpapi.WithIdempotency(fsstore.NewIdempotency(store)),
		httpapi.WithSharedLimiter(fsstore.NewRateLimiter(store)))
	if err != nil {
		return err
	}
	return listen(ctx, log, cfg.Port, api.Handler())
}

// newPayments chọn PayPal theo PAYPAL_MODE. off trả nil: các route thanh toán báo
// payment_not_enabled thay vì âm thầm giả lập.
func newPayments(cfg config.Config) (order.Payments, error) {
	switch cfg.PayPalMode {
	case "sandbox":
		return paypal.New(paypal.SandboxBaseURL, cfg.PayPalClientID, cfg.PayPalSecret)
	case "live":
		return paypal.New(paypal.LiveBaseURL, cfg.PayPalClientID, cfg.PayPalSecret)
	case "fake":
		if cfg.Env == "prod" {
			return nil, errors.New("PAYPAL_MODE=fake is not allowed in prod")
		}
		return paypal.NewFake(), nil
	}
	return nil, nil
}

// serveWorker: nhận task từ Cloud Tasks. Cloud Run đã kiểm tra OIDC (ingress internal,
// bắt buộc IAM), handler chỉ kiểm tra nội dung task.
func serveWorker(ctx context.Context, cfg config.Config, log *slog.Logger, store *fsstore.Store) error {
	auth, err := fbauth.New(ctx, cfg.ProjectID, false)
	if err != nil {
		return err
	}
	mailer, err := newMailer(cfg, log)
	if err != nil {
		return err
	}
	d := notify.NewDeliverer(fsstore.NewNotifyStore(store, auth), mailer)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /tasks/notify", notifyHandler(d, log))
	return listen(ctx, log, cfg.Port, mux)
}

// notifyHandler (B9): 2xx khi đã gửi hoặc lỗi vĩnh viễn (Cloud Tasks không thử lại), 503 khi
// lỗi tạm thời để Cloud Tasks thử lại theo cấu hình hàng đợi (giãn cách, số lần tối đa).
func notifyHandler(d *notify.Deliverer, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var t notify.Task
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&t); err != nil {
			log.WarnContext(r.Context(), "notify task: bad body", "err", err)
			w.WriteHeader(http.StatusNoContent) // task hỏng: thử lại cũng vô ích
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		retry, err := d.Deliver(ctx, t)
		attempt := r.Header.Get("X-CloudTasks-TaskRetryCount")
		if err != nil {
			log.WarnContext(ctx, "notify task", "event_id", t.EventID, "retry", retry, "attempt", attempt, "err", err)
		}
		if retry {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func runJob(ctx context.Context, name string, cfg config.Config, log *slog.Logger, store *fsstore.Store) error {
	start := time.Now()
	var err error
	switch name {
	case "digest":
		auth, aerr := fbauth.New(ctx, cfg.ProjectID, false)
		if aerr != nil {
			return aerr
		}
		mailer, merr := newMailer(cfg, log)
		if merr != nil {
			return merr
		}
		_, err = notify.NewDigestJob(fsstore.NewNotifyStore(store, auth), mailer, log, nil).Run(ctx)
	case "migrate":
		// A4: đưa dữ liệu mọi người dùng lên phiên bản mới; chạy sau mỗi lần deploy có bước mới.
		var n int
		n, err = store.MigrateAll(ctx, log)
		log.InfoContext(ctx, "migrate done", "users", n, "schema_version", fsstore.SchemaVersion)
	case "cleanup":
		var res fsstore.CleanupResult
		res, err = store.Cleanup(ctx, time.Now(), log)
		log.InfoContext(ctx, "cleanup done", "events", res.Events)
	case "fx-sync":
		var rates []fx.Rate
		rates, err = fx.Sync(ctx, fx.ERAPI{}, store, time.Now())
		log.InfoContext(ctx, "fx-sync done", "currencies", len(rates))
	case "seed-demo":
		// V5: ghi lịch sử mua mẫu cho tài khoản demo (đăng nhập tài khoản đó một lần trước).
		// Chỉ chạy khi đặt DEMO_UID, để không ai lỡ tay ghi vào tài khoản thật.
		uid := os.Getenv("DEMO_UID")
		if uid == "" {
			return errors.New("seed-demo: đặt DEMO_UID là uid tài khoản demo")
		}
		var rs *receipt.Service
		// Thiếu WithTaxonomy thì mọi danh mục bị coi là chưa phân loại và không vào tồn kho.
		if rs, err = receipt.New(nil, store, store, store, store, log, receipt.WithTaxonomy(groupTaxonomy.SpendingKeys())); err == nil {
			var n int
			n, err = demo.Seed(ctx, rs, uid, time.Now())
			log.InfoContext(ctx, "seed-demo done", "receipts", n)
		}
	case "price-sync":
		// ADR-004: bảng giá chung chỉ từ nguồn công khai. Thứ tự nguồn = thứ tự ưu tiên;
		// nguồn đối tác cắm thêm vào trước Gemini khi có hợp tác.
		if cfg.GeminiModel == "" {
			return errors.New("price-sync: thiếu GEMINI_MODEL")
		}
		var r *gemini.Reader
		if r, err = gemini.New(ctx, cfg.ProjectID, cfg.GeminiLocation, cfg.GeminiModel, nil, nil); err == nil {
			var res market.Result
			res, err = market.Sync(ctx, store, store, []market.Source{r.NewPriceSearch(store, log)},
				market.Options{Limit: cfg.PriceSyncLimit, Now: time.Now(), Log: log, FX: store})
			log.InfoContext(ctx, "price-sync done", "wanted", res.Wanted, "fresh", res.Fresh, "looked", res.Looked,
				"found", res.Found, "skipped", res.Skipped, "errors", res.Errors)
		}
	default:
		err = fmt.Errorf("unknown job %q", name)
	}
	// Một dòng log có cấu trúc cho mỗi lượt chạy; metric dựa trên log báo động khi
	// "lần thành công gần nhất" quá cũ.
	log.InfoContext(ctx, "job finished", "job", name, "ok", err == nil, "duration_ms", time.Since(start).Milliseconds())
	return err
}

// newMailer: chưa chọn nhà cung cấp email. Ở dev ghi log (không nội dung); ở prod từ
// chối khởi động thay vì âm thầm không gửi.
func newMailer(cfg config.Config, log *slog.Logger) (notify.Mailer, error) {
	if cfg.Env == "prod" {
		if err := cfg.RequireWorker(); err != nil {
			return nil, err
		}
		return nil, errors.New("email provider not configured")
	}
	return logMailer{log: log}, nil
}

type logMailer struct{ log *slog.Logger }

func (m logMailer) Send(ctx context.Context, _, subject string, lines []string) error {
	m.log.InfoContext(ctx, "dev mail", "subject", subject, "lines", len(lines))
	return nil
}

// listen chạy HTTP server với timeout đầy đủ và tắt êm khi nhận SIGTERM.
func listen(ctx context.Context, log *slog.Logger, port string, h http.Handler) error {
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second, // tải ảnh hóa đơn trên mạng di động chậm
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	log.Info("shutting down")
	return srv.Shutdown(shutdownCtx)
}
