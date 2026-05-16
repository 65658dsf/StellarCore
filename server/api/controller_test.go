package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	httppkg "github.com/65658dsf/StellarCore/pkg/util/http"
	utillog "github.com/65658dsf/StellarCore/pkg/util/log"
)

func performRequest(t *testing.T, handler httppkg.APIHandler, method, path string, body *strings.Reader) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	recorder := httptest.NewRecorder()
	httppkg.MakeHTTPHandlerFunc(handler).ServeHTTP(recorder, req)
	return recorder
}

func TestControllerHealthz(t *testing.T) {
	controller := NewController(ControllerParams{})
	recorder := performRequest(t, controller.Healthz, http.MethodGet, "/healthz", strings.NewReader(""))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", recorder.Body.String())
	}
}

func TestControllerGetConfig(t *testing.T) {
	controller := NewController(ControllerParams{
		ReadConfig: func() ([]byte, error) {
			return []byte("bindPort = 7000"), nil
		},
	})

	recorder := performRequest(t, controller.GetConfig, http.MethodGet, "/api/config", strings.NewReader(""))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "bindPort = 7000" {
		t.Fatalf("unexpected body %q", body)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", contentType)
	}
}

func TestControllerGetConfigError(t *testing.T) {
	controller := NewController(ControllerParams{
		ReadConfig: func() ([]byte, error) {
			return nil, httppkg.NewError(http.StatusBadRequest, "frps has no config file path")
		},
	})

	recorder := performRequest(t, controller.GetConfig, http.MethodGet, "/api/config", strings.NewReader(""))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "frps has no config file path") {
		t.Fatalf("unexpected response %q", recorder.Body.String())
	}
}

func TestControllerRestartService(t *testing.T) {
	called := 0
	controller := NewController(ControllerParams{
		RestartService: func() error {
			called++
			return nil
		},
	})

	recorder := performRequest(t, controller.RestartService, http.MethodPost, "/api/restart", strings.NewReader(""))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if called != 1 {
		t.Fatalf("expected restart callback to run once, got %d", called)
	}
}

func TestControllerRestartServiceConflict(t *testing.T) {
	controller := NewController(ControllerParams{
		RestartService: func() error {
			return httppkg.NewError(http.StatusConflict, "restart already in progress")
		},
	})

	recorder := performRequest(t, controller.RestartService, http.MethodPost, "/api/restart", strings.NewReader(""))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
}

func TestControllerGetLogs(t *testing.T) {
	utillog.ResetBufferForTesting()
	utillog.Infof("controller log test")
	controller := NewController(ControllerParams{})

	recorder := performRequest(t, controller.GetLogs, http.MethodGet, "/api/logs?limit=10", strings.NewReader(""))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var resp utillog.LogQueryResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(resp.Entries))
	}
	if !strings.Contains(resp.Entries[0].Message, "controller log test") {
		t.Fatalf("unexpected log entry %#v", resp.Entries[0])
	}
}

func TestControllerGetLogsRejectsInvalidCursor(t *testing.T) {
	controller := NewController(ControllerParams{})
	recorder := performRequest(t, controller.GetLogs, http.MethodGet, "/api/logs?cursor=bad", strings.NewReader(""))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestControllerUpdateService(t *testing.T) {
	called := false
	controller := NewController(ControllerParams{
		UpdateService: func(ctx context.Context) (UpdateResp, error) {
			called = true
			return UpdateResp{
				CurrentVersion: "1.1.6",
				LatestVersion:  "1.1.7",
				HasUpdate:      true,
				UpdateStarted:  true,
				Message:        "update started",
			}, nil
		},
	})

	recorder := performRequest(t, controller.UpdateService, http.MethodPost, "/api/update", strings.NewReader(""))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !called {
		t.Fatalf("expected update callback to be called")
	}

	var resp UpdateResp
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.HasUpdate || !resp.UpdateStarted || resp.LatestVersion != "1.1.7" {
		t.Fatalf("unexpected update response %#v", resp)
	}
}

func TestControllerUpdateServiceConflict(t *testing.T) {
	controller := NewController(ControllerParams{
		UpdateService: func(ctx context.Context) (UpdateResp, error) {
			return UpdateResp{}, httppkg.NewError(http.StatusConflict, "update already in progress")
		},
	})

	recorder := performRequest(t, controller.UpdateService, http.MethodPost, "/api/update", strings.NewReader(""))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
}

func TestControllerKickClientResponses(t *testing.T) {
	controller := NewController(ControllerParams{
		KickClient: func(runID string) (bool, error) {
			return runID == "run-ok", nil
		},
	})

	t.Run("invalid json", func(t *testing.T) {
		recorder := performRequest(t, controller.KickClient, http.MethodPost, "/api/client/kick", strings.NewReader("not-json"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected http 200, got %d", recorder.Code)
		}

		var resp CloseUserResp
		if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if resp.Status != http.StatusBadRequest {
			t.Fatalf("expected body status 400, got %d", resp.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		recorder := performRequest(t, controller.KickClient, http.MethodPost, "/api/client/kick", strings.NewReader(`{"runId":"run-missing"}`))
		var resp CloseUserResp
		if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if resp.Status != http.StatusNotFound {
			t.Fatalf("expected body status 404, got %d", resp.Status)
		}
	})

	t.Run("success", func(t *testing.T) {
		recorder := performRequest(t, controller.KickClient, http.MethodPost, "/api/client/kick", strings.NewReader(`{"runId":"run-ok"}`))
		var resp CloseUserResp
		if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if resp.Status != http.StatusOK || resp.Msg != "success" {
			t.Fatalf("unexpected response %#v", resp)
		}
	})
}

func TestControllerAPIAllProxiesTrafficEmpty(t *testing.T) {
	controller := NewController(ControllerParams{ServerCfg: &v1.ServerConfig{}})
	recorder := performRequest(t, controller.APIAllProxiesTraffic, http.MethodGet, "/api/traffic", strings.NewReader(""))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var resp GetProxyTrafficResp
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.TrafficIn) != 30 || len(resp.TrafficOut) != 30 {
		t.Fatalf("expected 30 traffic points, got %d/%d", len(resp.TrafficIn), len(resp.TrafficOut))
	}
}

func TestControllerAPITrafficTrendDay(t *testing.T) {
	controller := NewController(ControllerParams{ServerCfg: &v1.ServerConfig{}})
	recorder := performRequest(t, controller.APITrafficTrend, http.MethodGet, "/api/traffic/trend?range=day", strings.NewReader(""))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var resp TrafficTrendResp
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Timestamps) != 24 || len(resp.InData) != 24 || len(resp.OutData) != 24 {
		t.Fatalf("expected 24 points, got %d/%d/%d", len(resp.Timestamps), len(resp.InData), len(resp.OutData))
	}
}
