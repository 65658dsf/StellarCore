package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	httppkg "github.com/65658dsf/StellarCore/pkg/util/http"
	"github.com/65658dsf/StellarCore/pkg/util/xlog"
	"github.com/65658dsf/StellarCore/server/api"
	"github.com/65658dsf/StellarCore/server/registry"
)

func TestControlManagerDelDoesNotRemoveReplacedControl(t *testing.T) {
	oldConn, oldPeer := net.Pipe()
	newConn, newPeer := net.Pipe()
	defer oldPeer.Close()
	defer newConn.Close()
	defer newPeer.Close()

	oldCtl := &Control{conn: oldConn, xl: xlog.New()}
	newCtl := &Control{conn: newConn, xl: xlog.New()}

	manager := NewControlManager()
	manager.Add("run-1", oldCtl)
	manager.Add("run-1", newCtl)

	clientRegistry := registry.NewClientRegistry()
	clientRegistry.Register("", "", "run-1", "host-a", "127.0.0.1")

	if manager.Del("run-1", oldCtl) {
		clientRegistry.MarkOfflineByRunID("run-1")
	}

	if _, ok := clientRegistry.GetByRunID("run-1"); !ok {
		t.Fatalf("expected replaced control to keep client online")
	}
}

func TestControlManagerDelRemovesCurrentControl(t *testing.T) {
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()

	ctl := &Control{conn: conn, xl: xlog.New()}
	manager := NewControlManager()
	manager.Add("run-2", ctl)

	clientRegistry := registry.NewClientRegistry()
	clientRegistry.Register("", "", "run-2", "host-b", "127.0.0.1")

	if manager.Del("run-2", ctl) {
		clientRegistry.MarkOfflineByRunID("run-2")
	}

	if _, ok := clientRegistry.GetByRunID("run-2"); ok {
		t.Fatalf("expected current control removal to mark client offline")
	}
}

func TestServiceRegisterRouteHandlersUsesAPIController(t *testing.T) {
	cfg := &v1.ServerConfig{}
	controller := api.NewController(api.ControllerParams{
		ServerCfg: cfg,
		ReadConfig: func() ([]byte, error) {
			return []byte("bindPort = 7000"), nil
		},
	})

	service := &Service{
		cfg:           cfg,
		apiController: controller,
	}

	router := mux.NewRouter()
	service.registerRouteHandlers(&httppkg.RouterRegisterHelper{
		Router:         router,
		AssetsFS:       http.Dir("."),
		AuthMiddleware: mux.MiddlewareFunc(func(next http.Handler) http.Handler { return next }),
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "bindPort = 7000" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestServiceScheduleRestartUsesExecutorAndRecoversAfterError(t *testing.T) {
	executed := make(chan struct{}, 2)
	service := &Service{
		restartExecutor: func() error {
			executed <- struct{}{}
			return fmt.Errorf("boom")
		},
	}

	if err := service.scheduleRestart(); err != nil {
		t.Fatalf("first restart should be accepted: %v", err)
	}

	if err := service.scheduleRestart(); err == nil {
		t.Fatalf("expected immediate second restart to conflict")
	}

	select {
	case <-executed:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected restart executor to run")
	}

	time.Sleep(50 * time.Millisecond)
	if err := service.scheduleRestart(); err != nil {
		t.Fatalf("expected restart flag to reset after executor error: %v", err)
	}
}
