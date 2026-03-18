package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	reqmodel "github.com/easy-cloud-Knet/KWS_Control/request/model"
	"github.com/easy-cloud-Knet/KWS_Control/service"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/redis/go-redis/v9"
)

func TestVMStartStatusShutdown_WithSimulatedCoreAndRedis(t *testing.T) {
	t.Parallel()

	vmUUID := structure.UUID("a4b719aa-7f77-4b7f-8f45-e8470bd7e5d0")

	coreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/BOOTVM":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"BootVM operation success"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/getStatusUUID":
			var req reqmodel.GetVMStatusRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.UUID != vmUUID || req.DataType != reqmodel.CpuInfo {
				http.Error(w, "unexpected payload", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"system_time":1.2,"idle_time":98.8,"usage_percent":15.5}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/forceShutDownUUID":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Force shutdown success"}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer coreServer.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	core := mustParseCoreFromServerURL(t, coreServer.URL)
	core.VMInfoIdx = map[structure.UUID]*structure.VMInfo{
		vmUUID: {
			UUID: vmUUID,
		},
	}

	ctx := structure.ControlContext{
		DB:     db,
		Cores:  []structure.Core{core},
		AliveVM: []*structure.VMInfo{
			{UUID: vmUUID},
		},
	}

	if err := service.StoreVMInfoToRedis(context.Background(), rdb, reqmodel.VMRedisInfo{
		UUID:   vmUUID,
		CPU:    2,
		Memory: 2048,
		Disk:   30,
		IP:     "10.0.0.11",
		Status: reqmodel.VMStatusStartBegin,
		Time:   time.Now().Unix(),
	}); err != nil {
		t.Fatalf("failed to seed redis vm info: %v", err)
	}

	srv := newAPIIntegrationServer(t, &ctx, rdb)

	expectVMCoreLookup(sqlMock, vmUUID, 0)
	startResp := doJSONRequest(t, srv, http.MethodPost, "/vm/start", map[string]any{"uuid": vmUUID})
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/start status: got=%d body=%s", startResp.StatusCode, startResp.BodyString)
	}

	expectVMCoreLookup(sqlMock, vmUUID, 0)
	statusResp := doJSONRequest(t, srv, http.MethodGet, "/vm/status", map[string]any{"uuid": vmUUID, "type": "cpu"})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/status status: got=%d body=%s", statusResp.StatusCode, statusResp.BodyString)
	}
	if gotUsage, ok := statusResp.JSONBody["usage_percent"].(float64); !ok || gotUsage != 15.5 {
		t.Fatalf("unexpected /vm/status response: %+v", statusResp.JSONBody)
	}

	expectVMCoreLookup(sqlMock, vmUUID, 0)
	shutdownResp := doJSONRequest(t, srv, http.MethodPost, "/vm/shutdown", map[string]any{"uuid": vmUUID})
	if shutdownResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/shutdown status: got=%d body=%s", shutdownResp.StatusCode, shutdownResp.BodyString)
	}

	stored, err := service.GetVMInfoFromRedis(context.Background(), rdb, vmUUID)
	if err != nil {
		t.Fatalf("failed to read redis value after shutdown: %v", err)
	}
	if stored.Status != reqmodel.VMStatusStopped {
		t.Fatalf("unexpected redis status after shutdown: got=%s want=%s", stored.Status, reqmodel.VMStatusStopped)
	}
	if len(ctx.AliveVM) != 0 {
		t.Fatalf("alive vm list should be empty after shutdown, got=%d", len(ctx.AliveVM))
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations not met: %v", err)
	}
}

func TestVMConnect_WithSimulatedGuacamole(t *testing.T) {
	t.Parallel()

	vmUUID := structure.UUID("499cb4fd-1d0d-442e-8945-9e39a2eb0d44")
	guacPassword := "mock-guac-password"

	guacServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tokens" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("username") != string(vmUUID) || r.Form.Get("password") != guacPassword {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authToken":"mock-auth-token","username":"ok","dataSource":"mysql","availableDataSources":["mysql"]}`))
	}))
	defer guacServer.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	core := structure.Core{
		IP:   "127.0.0.1",
		Port: 9999,
		VMInfoIdx: map[structure.UUID]*structure.VMInfo{
			vmUUID: {
				UUID:         vmUUID,
				GuacPassword: guacPassword,
			},
		},
	}

	ctx := structure.ControlContext{
		DB:     db,
		Config: structure.Config{GuacBaseURL: guacServer.URL},
		Cores:  []structure.Core{core},
	}
	srv := newAPIIntegrationServer(t, &ctx, rdb)

	expectVMCoreLookup(sqlMock, vmUUID, 0)
	resp := doNoBodyRequest(t, srv, http.MethodGet, "/vm/connect?uuid="+string(vmUUID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/connect status: got=%d body=%s", resp.StatusCode, resp.BodyString)
	}
	if got, ok := resp.JSONBody["authToken"].(string); !ok || got != "mock-auth-token" {
		t.Fatalf("unexpected /vm/connect response: %+v", resp.JSONBody)
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations not met: %v", err)
	}
}

func TestVMRedisAndVMInfo_NormalizedAndReadable(t *testing.T) {
	t.Parallel()

	vmUUID := structure.UUID("f63d5982-410d-4e56-9b32-f0f661997d62")

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := service.StoreVMInfoToRedis(context.Background(), rdb, reqmodel.VMRedisInfo{
		UUID:   vmUUID,
		CPU:    4,
		Memory: 4096,
		Disk:   60,
		IP:     "10.20.0.8",
		Status: reqmodel.VMStatusPrepareBegin,
		Time:   1700000000,
	}); err != nil {
		t.Fatalf("failed to seed redis vm info: %v", err)
	}

	ctx := structure.ControlContext{
		DB:    db,
		Cores: []structure.Core{{}},
	}
	srv := newAPIIntegrationServer(t, &ctx, rdb)

	redisResp := doJSONRequest(t, srv, http.MethodPost, "/vm/redis", map[string]any{
		"UUID":   vmUUID,
		"status": "unexpected-status-value",
	})
	if redisResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/redis status: got=%d body=%s", redisResp.StatusCode, redisResp.BodyString)
	}

	vmInfoResp := doJSONRequest(t, srv, http.MethodGet, "/vm/info", map[string]any{"uuid": vmUUID})
	if vmInfoResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /vm/info status: got=%d body=%s", vmInfoResp.StatusCode, vmInfoResp.BodyString)
	}

	if gotStatus, err := service.GetVMInfoFromRedis(context.Background(), rdb, vmUUID); err != nil {
		t.Fatalf("failed to load vm from redis: %v", err)
	} else if gotStatus.Status != reqmodel.VMStatusUnknown {
		t.Fatalf("expected normalized redis status %q, got=%q", reqmodel.VMStatusUnknown, gotStatus.Status)
	}

	if gotUUID, ok := vmInfoResp.JSONBody["uuid"].(string); !ok || gotUUID != string(vmUUID) {
		t.Fatalf("unexpected /vm/info response: %+v", vmInfoResp.JSONBody)
	}
}

type testHTTPResponse struct {
	StatusCode int
	BodyString string
	JSONBody   map[string]any
}

func newAPIIntegrationServer(t *testing.T, controlCtx *structure.ControlContext, rdb *redis.Client) *httptest.Server {
	t.Helper()

	h := handlerContext{
		context: controlCtx,
		rdb:     rdb,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /vm/start", h.startVm)
	mux.HandleFunc("POST /vm/shutdown", h.shutdownVm)
	mux.HandleFunc("GET /vm/status", h.vmStatus)
	mux.HandleFunc("GET /vm/connect", h.vmConnect)
	mux.HandleFunc("POST /vm/redis", h.redis)
	mux.HandleFunc("GET /vm/info", h.vmInfo)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func doJSONRequest(t *testing.T, srv *httptest.Server, method, path string, payload map[string]any) testHTTPResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	out := testHTTPResponse{
		StatusCode: resp.StatusCode,
		BodyString: string(respBytes),
	}

	var obj map[string]any
	if len(respBytes) > 0 && json.Unmarshal(respBytes, &obj) == nil {
		out.JSONBody = obj
	}

	return out
}

func doNoBodyRequest(t *testing.T, srv *httptest.Server, method, path string) testHTTPResponse {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	out := testHTTPResponse{
		StatusCode: resp.StatusCode,
		BodyString: string(respBytes),
	}
	var obj map[string]any
	if len(respBytes) > 0 && json.Unmarshal(respBytes, &obj) == nil {
		out.JSONBody = obj
	}
	return out
}

func expectVMCoreLookup(mock sqlmock.Sqlmock, uuid structure.UUID, coreIndex int) {
	rows := sqlmock.NewRows([]string{"core"}).AddRow(coreIndex)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT core FROM inst_loc WHERE uuid = \\?").WithArgs(uuid).WillReturnRows(rows)
	mock.ExpectCommit()
}

func mustParseCoreFromServerURL(t *testing.T, serverURL string) structure.Core {
	t.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse test server url: %v", err)
	}
	host, portStr, err := splitHostPort(u.Host)
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port in test server url: %v", err)
	}

	return structure.Core{
		IP:   host,
		Port: uint16(port),
	}
}

func splitHostPort(hostport string) (string, string, error) {
	u, err := url.Parse("http://" + hostport)
	if err != nil {
		return "", "", err
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" || port == "" {
		return "", "", fmt.Errorf("host or port missing in %q", hostport)
	}
	return host, port, nil
}
