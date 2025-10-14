package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/api/model"
	"github.com/easy-cloud-Knet/KWS_Control/service"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func (c *handlerContext) createVm(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()

	err := service.CreateVM(w, r, c.context, c.rdb)
	if err != nil {
		h := w.Header()
		h.Del("Content-Length")
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusMethodNotAllowed)

		log.Error("Failed to create VM: %v", err, true)

		return
	}
	defer r.Body.Close()

	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("VM created successfully"))
}

func (c *handlerContext) deleteVm(w http.ResponseWriter, r *http.Request) {
	var req model.ApiDeleteVmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	err := service.DeleteVM(req.UUID, c.context, c.rdb)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError) // TODO: 코어가 없는 경우 처리
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *handlerContext) shutdownVm(w http.ResponseWriter, r *http.Request) {
	var req model.ApiShutdownVmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	err := service.ShutdownVM(req.UUID, c.context, c.rdb)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *handlerContext) vmStatus(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()

	var req model.ApiVmStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Error("Failed to decode request body: %v", err, true)
		return
	}
	defer r.Body.Close()

	statusType := req.Type
	if statusType != "cpu" && statusType != "memory" && statusType != "disk" {
		http.Error(w, "Invalid status type. Must be 'cpu', 'memory', or 'disk'", http.StatusBadRequest)
		return
	}

	var data any
	var err error

	switch statusType {
	case "cpu":
		data, err = service.GetVMCpuInfo(req.UUID, c.context)
	case "memory":
		data, err = service.GetVMMemoryInfo(req.UUID, c.context)
	case "disk":
		data, err = service.GetVMDiskInfo(req.UUID, c.context)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error("Failed to get VM status: %v", err, true)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error("Failed to encode response: %v", err, true)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (c *handlerContext) vmConnect(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()

	uuidStr := r.URL.Query().Get("uuid")
	if uuidStr == "" {
		http.Error(w, "Missing 'uuid' query parameter", http.StatusBadRequest)
		log.Error("Missing 'uuid' query parameter", nil, true)
		return
	}

	var uuid = structure.UUID(uuidStr)

	authToken, err := service.GetGuacamoleToken(uuid, c.context)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error("Failed to get Guacamole token: %v", err, true)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"authToken": authToken}); err != nil {
		log.Error("Failed to encode response: %v", err, true)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
func (c *handlerContext) redis(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()

	var req model.Redis
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Error("Invalid request body: %v", err, true)
		return
	}
	defer r.Body.Close()
	originalStatus := req.Status
	normalizedStatus := model.ValidateAndNormalizeStatus(req.Status)

	if originalStatus != normalizedStatus {
		log.Warn("VM status normalized: UUID=%s, original='%s', normalized='%s'",
			req.UUID, originalStatus, normalizedStatus, true)
	}

	ctx := r.Context()

	err := service.UpdateVMStatusInRedis(ctx, c.rdb, req.UUID, normalizedStatus, time.Now().Unix())
	if err != nil {
		http.Error(w, "Failed to update VM status in Redis", http.StatusInternalServerError)
		log.Error("Failed to update VM status in Redis: %v", err, true)
		return
	}

	storedValue, err := c.rdb.Get(ctx, string(req.UUID)).Result()
	if err != nil {
		http.Error(w, "Failed to get value from Redis", http.StatusInternalServerError)
		log.Error("Redis GET failed: %v", err, true)
		return
	}

	log.DebugInfo("Redis 확인 완료 - key: %s, value: %s", req.UUID, storedValue)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("VM status updated in Redis"))
}

func (c *handlerContext) vmInfo(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()

	var req model.ApiVmInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		log.Error("Invalid request body: %v", err, true)
		return
	}
	defer r.Body.Close()

	vmInfo, err := service.GetVMInfoFromRedis(r.Context(), c.rdb, req.UUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		log.Error("failed to get vm info from redis: %v", err, true)
		return
	}

	response := model.ApiVmInfoResponse{
		UUID:   vmInfo.UUID,
		CPU:    vmInfo.CPU,
		Memory: vmInfo.Memory,
		Disk:   vmInfo.Disk,
		IP:     vmInfo.IP,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("failed to encode vm info response: %v", err, true)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	log.Info("retrieved vm info from redis: UUID=%s", string(req.UUID), true)
}
