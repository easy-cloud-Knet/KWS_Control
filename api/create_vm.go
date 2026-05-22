package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/service"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func (c *handlerContext) createVm(w http.ResponseWriter, r *http.Request) {
	log := util.GetLogger()
	defer r.Body.Close()

	var req model.CreateVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("createVm: failed to parse request body: %v", err, true)
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			util.RespondError(w, http.StatusBadRequest, "invalid JSON format in request body")
		} else {
			util.RespondError(w, http.StatusBadRequest, "err req body parsing: "+err.Error())
		}
		return
	}

	if req.HardwareInfo.Memory == 0 || req.HardwareInfo.CPU == 0 || req.HardwareInfo.Disk == 0 {
		util.RespondError(w, http.StatusBadRequest, "Memory, CPU, and Disk must be non-zero")
		return
	}

	err := service.CreateVM(req, c.context, c.rdb)
	if err != nil {
		log.Error("createVm: failed to create VM: %v", err, true)
		util.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	util.RespondJSON(w, http.StatusCreated, map[string]string{"message": "VM created successfully"})
}
