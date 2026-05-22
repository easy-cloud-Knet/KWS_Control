package api

import (
	"encoding/json"
	"net/http"

	"github.com/easy-cloud-Knet/KWS_Control/service"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

type ApiVmStatusRequest struct {
	UUID structure.UUID `json:"uuid"`
	Type string         `json:"type"` // "cpu", "memory", or "disk"
}

func (c *handlerContext) vmStatus(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()
	defer r.Body.Close()

	var req ApiVmStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.RespondError(w, http.StatusBadRequest, "Invalid request body")
		log.Error("Failed to decode request body: %v", err, true)
		return
	}

	statusType := req.Type
	if statusType != "cpu" && statusType != "memory" && statusType != "disk" {
		util.RespondError(w, http.StatusBadRequest, "Invalid status type. Must be 'cpu', 'memory', or 'disk'")
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
		log.Error("Failed to get VM status: %v", err, true)
		util.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	util.RespondJSON(w, http.StatusOK, data)
}
