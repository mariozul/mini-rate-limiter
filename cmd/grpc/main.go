// Package main is the composition root for the astro-boilerplate gRPC service.
//
// Every adapter is constructed exactly once here and threaded through the
// application via constructor injection. main.go owns the lifecycle of every
// external resource (databases, Pub/Sub clients, OS signals).
//
// Wiring order:
//  1. Configuration: cfg := config.Load() (env-driven; mandatory vars fatal).
//  2. Logger: log := logger.NewLogger().
//  3. Signal-cancellable context (signal.NotifyContext) + ShutdownManager.
//     The ctx propagates to every long-lived resource so a SIGTERM/SIGINT
//     unblocks them naturally. The manager owns ordered, per-phase teardown.
//  4. Datadog tracer start.
//  5. Postgres master + replica (safesql-backed; read/write split enforced at the type level).
//  6. Optional Redis distributed lock; cache.NewNoOp() when redis.url is empty.
//  7. Optional MongoDB audit-log writer; vendor_audit.NewNoOp() when mongo.uri is empty.
//  8. Vendor Postgres repository (read/write split).
//  9. Transaction manager bound to the master.
//  10. Optional upstream gRPC clients (one ClientConn per upstream); skipped
//      when grpc_service.<entry>.addr is empty.
//  11. Pub/Sub client (astro-golibs) + publisher + optional subscriber.
//  12. Application handlers (CreateVendor command + GetVendor query).
//  13. Interface (gRPC) handlers: vendors/v1 (erp.VendorService) + health/v1.
//      The health handler holds the DB handles directly and verifies them
//      with SELECT 1 on every Check.
//  14. gRPC server + metrics HTTP server bootstrap.
//  15. Register ordered shutdown phases (readiness → grpc → upstreams →
//      metrics → mongo → redis → postgres → tracer).
//  16. Wait for SIGTERM/SIGINT or fatal serve error; unregister the signal
//      handler so a second signal hits the OS default (force-exit); run
//      the shutdown manager.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	gcpubsub "cloud.google.com/go/pubsub/v2"
	logger "github.com/astronautsid/astro-golibs/logger"
	golibspubsub "github.com/astronautsid/astro-golibs/pubsub/v2"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"

	"github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/astronautsid/astro-boilerplate/internal/config"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/cache"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/external"
	extboilerplate "github.com/astronautsid/astro-boilerplate/internal/infrastructure/external/boilerplate"
	messagingpubsub "github.com/astronautsid/astro-boilerplate/internal/infrastructure/messaging/pubsub"
	vendorpub "github.com/astronautsid/astro-boilerplate/internal/infrastructure/messaging/pubsub/vendors"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/mongodb"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/mongodb/vendor_audit"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/persistence"
	vendorrepo "github.com/astronautsid/astro-boilerplate/internal/infrastructure/persistence/vendors"
	grpc_interface "github.com/astronautsid/astro-boilerplate/internal/interface/grpc"
	healthv1 "github.com/astronautsid/astro-boilerplate/internal/interface/grpc/handler/health/v1"
	vendorv1 "github.com/astronautsid/astro-boilerplate/internal/interface/grpc/handler/vendors/v1"
	vendoreventsv1 "github.com/astronautsid/astro-boilerplate/internal/interface/pubsub/vendors/v1"
	"github.com/astronautsid/astro-boilerplate/pkg/shutdown"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// pubsubVendorTopic is the Pub/Sub topic name used for vendor-activated
// events. Hardcoded for the boilerplate; concrete services SHOULD promote
// this to an env var.
const pubsubVendorTopic = "vendor-notifications"

// preStopSleep is the readiness-flip-to-listener-close gap. The article
// calls this the "pre-stop sleep": after we mark the health handler
// NOT_SERVING, the load balancer needs a few seconds to delist the pod
// before we close the listener. Five seconds covers the default kube-proxy
// sync interval; concrete services may tune this against their LB.
const preStopSleep = 5 * time.Second

func main() {
	// --- Step 1. Configuration --------------------------------------------
	cfg := config.Load()

	// --- Step 2. Logger ---------------------------------------------------
	// astro-golibs/logger emits JSON to stdout with caller info, level
	// driven by cfg.Log.Level ("debug" / "info" / "warn" / "error").
	appLog, err := logger.InitLogger(cfg.Log.Level)
	if err != nil {
		log.Fatalf("logger init: %v", err)
	}
	appLog.Info("starting astro-boilerplate gRPC service")

	// --- Step 3. Signal context + shutdown manager ------------------------
	// signal.NotifyContext returns a ctx that cancels on SIGTERM/SIGINT,
	// plus a stop fn that un-registers the signal handler. We pass ctx
	// through to every long-lived resource (Pub/Sub subscriber, Mongo
	// connect probe, etc.) so signal-cancellation propagates naturally.
	// stopSignals() is called explicitly right before we run shutdown so
	// a second signal hits OS default (immediate termination) — the
	// article's "double-signal" pattern.
	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	sm := shutdown.NewManager()

	// --- Step 4. Datadog tracer -------------------------------------------
	// Tracing is observability, not a critical path — if the agent is
	// unreachable we log and continue rather than failing startup.
	if err := tracer.Start(
		tracer.WithAgentAddr(cfg.Datadog.AgentAddr),
		tracer.WithService(cfg.App.Name),
	); err != nil {
		appLog.Warnf("datadog tracer start: %v", err)
	}

	// --- Step 5. Postgres master + replica --------------------------------
	writeDB, err := persistence.NewMasterDB(cfg.Postgres.Master, cfg.App.Name)
	if err != nil {
		log.Fatalf("postgres master init: %v", err)
	}
	readDB, err := persistence.NewReplicaDB(cfg.Postgres.Slave, cfg.App.Name)
	if err != nil {
		log.Fatalf("postgres replica init: %v", err)
	}

	// --- Step 6. Distributed lock (Redis optional) ------------------------
	var (
		distributedLock vendors.DistributedLock
		redisClient     *goredis.Client
	)
	if cfg.Redis.URL != "" {
		redisClient, err = cache.NewRedisDB(cfg.Redis)
		if err != nil {
			log.Fatalf("redis init: %v", err)
		}
		distributedLock = cache.NewLock(redisClient)
		appLog.Info("redis-backed distributed lock enabled")
	} else {
		distributedLock = cache.NewNoOp()
		appLog.Info("no redis.url set; falling back to NoOp distributed lock")
	}

	// --- Step 7. Audit log writer (Mongo optional) ------------------------
	var (
		auditLog vendors.AuditLogWriter
		mongoDB  *mongo.Database
	)
	if cfg.Mongo.URI != "" {
		mongoDB, err = mongodb.NewMongoDB(ctx, cfg.Mongo)
		if err != nil {
			if errors.Is(err, mongodb.ErrDisabled) {
				mongoDB = nil
				auditLog = vendor_audit.NewNoOp()
				appLog.Info("mongo disabled; falling back to NoOp audit log writer")
			} else {
				log.Fatalf("mongo init: %v", err)
			}
		} else {
			auditLog = vendor_audit.NewRepository(mongoDB)
			appLog.Info("mongo-backed audit log writer enabled")
		}
	} else {
		auditLog = vendor_audit.NewNoOp()
		appLog.Info("no mongo.uri set; falling back to NoOp audit log writer")
	}

	// --- Step 8. Vendor repository ----------------------------------------
	vendorRepo := vendorrepo.NewRepository(writeDB, readDB)

	// --- Step 9. Transaction manager --------------------------------------
	txMgr := persistence.NewTransactionManager(writeDB)

	// --- Step 10. External upstream gRPC clients (optional) ---------------
	var extBoilerplateConn *grpc.ClientConn
	if cfg.GrpcService.Boilerplate.Addr != "" {
		extBoilerplateConn, err = external.NewGrpcClientConn(ctx, cfg.GrpcService.Boilerplate, cfg.App.Name)
		if err != nil {
			log.Fatalf("dial boilerplate upstream: %v", err)
		}
		_ = extboilerplate.NewClient(extBoilerplateConn, cfg.GrpcService.Boilerplate.TimeoutSec)
		appLog.Info(fmt.Sprintf("boilerplate upstream client wired to %s", cfg.GrpcService.Boilerplate.Addr))
	} else {
		appLog.Info("no grpc_service.boilerplate.addr set; example external client skipped")
	}

	// --- Step 11. Pub/Sub client + publisher + subscriber -----------------
	var notificationSvc vendors.NotificationService
	var pubsubSvc interface {
		Publish(ctx context.Context, topic string, msg golibspubsub.Message) (string, error)
		RegisterSubscription(ctx context.Context, cfg golibspubsub.SubscriptionConfig, fn func(context.Context, *gcpubsub.Message)) error
	}
	if cfg.Gcp.ProjectID != "" {
		svc, err := golibspubsub.New(ctx, cfg.Gcp.ProjectID)
		if err != nil {
			log.Fatalf("pubsub client init: %v", err)
		}
		pubsubSvc = svc
		notificationSvc = vendorpub.NewPublisher(svc, pubsubVendorTopic)
		appLog.Info(fmt.Sprintf("pubsub publisher wired to topic %q", pubsubVendorTopic))
	} else {
		notificationSvc = vendorpub.NewNoopPublisher()
		appLog.Info("no gcp.project_id set; falling back to NoOp vendor notification publisher")
	}

	// The subscriber goroutine inside astro-golibs is bound to ctx, so
	// signal cancellation stops it from pulling new messages. But the
	// library spawns its receive loop internally with no Wait handle, so
	// we cannot observe when in-flight callbacks finish. To drain
	// cleanly before tearing down DBs we wrap the listener with our own
	// sync.WaitGroup; the pubsub-subscriber shutdown phase (registered
	// further below) waits on it.
	var (
		subWG         sync.WaitGroup
		subRegistered bool
	)
	if cfg.PubSub.VendorEventsSubscriptionID != "" && pubsubSvc != nil {
		subscriber := messagingpubsub.NewSubscriber(pubsubSvc)
		listener := vendoreventsv1.NewEventHandler(appLog)
		drainable := func(callCtx context.Context, msg *gcpubsub.Message) {
			subWG.Add(1)
			defer subWG.Done()
			listener.Handle(callCtx, msg)
		}
		if err := subscriber.Register(ctx, cfg.PubSub.VendorEventsSubscriptionID, drainable); err != nil {
			log.Fatalf("pubsub subscription register: %v", err)
		}
		subRegistered = true
		appLog.Info(fmt.Sprintf("pubsub subscription started: sub_id=%q", cfg.PubSub.VendorEventsSubscriptionID))
	} else {
		appLog.Info("no pubsub.vendor_events_subscription_id set; subscription disabled")
	}

	// --- Step 12. Application handlers ------------------------------------
	createHandler := vendors.NewCreateVendorHandler(vendors.CreateVendorParams{
		Repo:      vendorRepo,
		TxManager: txMgr,
		Lock:      distributedLock,
		AuditLog:  auditLog,
		Notify:    notificationSvc,
	})
	getHandler := vendors.NewGetVendorHandler(vendors.GetVendorParams{Repo: vendorRepo})

	// --- Step 13. Interface handlers --------------------------------------
	vendorHandler := vendorv1.NewHandler(createHandler, getHandler, appLog)
	healthHandler := healthv1.NewHandler(writeDB, readDB, appLog)

	// --- Step 14. Server bootstrap ----------------------------------------
	srv := grpc_interface.NewServer(appLog, cfg.App.Name, vendorHandler, healthHandler, cfg.Grpc.EnableReflection)

	lis, err := net.Listen("tcp", cfg.Grpc.Addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.Grpc.Addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		appLog.Info(fmt.Sprintf("gRPC server listening on %s", cfg.Grpc.Addr))
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{Addr: cfg.Http.Addr, Handler: metricsMux}
	go func() {
		appLog.Info(fmt.Sprintf("metrics HTTP server listening on %s", cfg.Http.Addr))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLog.Error(fmt.Sprintf("metrics server failed: %v", err))
		}
	}()

	// --- Step 15. Register shutdown phases (in execution order) -----------
	// Phase 1: Stop accepting new work — flip readiness and wait for the
	// load balancer to delist this pod before we close the listener.
	sm.Register("readiness", preStopSleep+time.Second, func(phaseCtx context.Context) error {
		healthHandler.MarkUnready()
		appLog.Info(fmt.Sprintf("health flipped to NOT_SERVING; sleeping %s for LB propagation", preStopSleep))
		select {
		case <-time.After(preStopSleep):
		case <-phaseCtx.Done():
		}
		return nil
	})

	// Phase 2: Drain in-flight work — GracefulStop waits for active RPCs
	// up to its own budget, then force-closes via srv.Stop() if exceeded.
	sm.Register("grpc-server", 15*time.Second, func(phaseCtx context.Context) error {
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
			return nil
		case <-phaseCtx.Done():
			srv.Stop()
			<-done
			return fmt.Errorf("graceful stop exceeded budget: %w", phaseCtx.Err())
		}
	})

	// Pub/Sub callback drain. ctx was already cancelled when the signal
	// fired, so astro-golibs has stopped pulling new messages and GCP's
	// Subscription.Receive is unwinding. The WaitGroup count reaches
	// zero once every callback that started before cancel has returned.
	// We block on it here so the next phases (DB closes) only fire after
	// every in-flight handler has released its DB connection.
	if subRegistered {
		sm.Register("pubsub-subscriber", 10*time.Second, func(phaseCtx context.Context) error {
			done := make(chan struct{})
			go func() {
				subWG.Wait()
				close(done)
			}()
			select {
			case <-done:
				return nil
			case <-phaseCtx.Done():
				return fmt.Errorf("pubsub drain exceeded budget: %w", phaseCtx.Err())
			}
		})
	}

	if extBoilerplateConn != nil {
		sm.Register("external-boilerplate", 3*time.Second, func(_ context.Context) error {
			return extBoilerplateConn.Close()
		})
	}

	sm.Register("metrics-server", 5*time.Second, func(phaseCtx context.Context) error {
		return metricsSrv.Shutdown(phaseCtx)
	})

	// Phase 3: Flush and close resources. Order matters within the
	// "close" phase too — close the message-store client before its
	// underlying connections, and Datadog last so we keep emitting
	// spans for the prior closes.
	if mongoDB != nil {
		sm.Register("mongo", 5*time.Second, func(phaseCtx context.Context) error {
			return mongoDB.Client().Disconnect(phaseCtx)
		})
	}
	if redisClient != nil {
		sm.Register("redis", 3*time.Second, func(_ context.Context) error {
			return redisClient.Close()
		})
	}
	sm.Register("postgres-master", 3*time.Second, func(_ context.Context) error {
		return writeDB.Close()
	})
	sm.Register("postgres-replica", 3*time.Second, func(_ context.Context) error {
		return readDB.Close()
	})
	sm.Register("tracer", 2*time.Second, func(_ context.Context) error {
		tracer.Stop()
		return nil
	})

	// --- Step 16. Wait for shutdown signal --------------------------------
	exitCode := 0
	select {
	case err := <-serveErr:
		if err != nil {
			appLog.Error(fmt.Sprintf("grpc server failed: %v", err))
			exitCode = 1
		}
	case <-ctx.Done():
		appLog.Info("shutdown initiated; a second signal will force-exit")
	}

	// Un-register signal.NotifyContext so a subsequent SIGTERM reverts to
	// the OS default (immediate process termination). Without this, a
	// stuck shutdown phase would leave the operator unable to kill the
	// process short of SIGKILL.
	stopSignals()

	if err := sm.Run(appLog); err != nil {
		appLog.Error(fmt.Sprintf("shutdown completed with errors: %v", err))
		exitCode = 1
	}
	appLog.Info("astro-boilerplate gRPC service stopped")
	if exitCode != 0 {
		// log.Fatal would skip the manager we just ran; we already ran it,
		// so just signal the exit code to the harness.
		log.SetFlags(0)
		log.Fatalf("exit %d", exitCode)
	}
}
