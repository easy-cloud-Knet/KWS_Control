package model

import "github.com/easy-cloud-Knet/KWS_Control/structure"

type ApiDeleteVmRequest struct {
	UUID structure.UUID `json:"uuid"`
}

type ApiShutdownVmRequest struct {
	UUID structure.UUID `json:"uuid"`
}
type ApiShutdownVmResponse struct {
	Message string `json:"message"`
}
type ApiForceShutdownVmRequest struct {
	UUID structure.UUID `json:"uuid"`
}
type ApiForceShutdownVmResponse struct {
	Message string `json:"message"`
}
type ApiStartVmRequest struct {
	UUID structure.UUID `json:"uuid"`
}

type ApiVmStatusRequest struct {
	UUID structure.UUID `json:"uuid"`
	Type string         `json:"type"` // "cpu", "memory", or "disk"
}

type ApiVmConnectRequest struct {
	UUID structure.UUID `json:"uuid"`
}

type ApiVmInfoRequest struct {
	UUID structure.UUID `json:"uuid"`
}

type ApiVmInfoResponse struct {
	UUID   structure.UUID `json:"uuid"`
	CPU    uint32         `json:"cpu"`
	Memory uint32         `json:"memory"` // MiB
	Disk   uint32         `json:"disk"`   // MiB
	IP     string         `json:"ip"`
}

// request/model/vm.go 와 동일하게 유지
const (
	VMStatusPrepareBegin = "prepare begin"
	VMStatusStartBegin   = "start begin"
	VMStatusStarted      = "started begin"
	VMStatusStopped      = "stopped end"
	VMStatusRelease      = "release end"
	VMStatusMigrate      = "migrate begin"
	VMStatusRestore      = "restort begin"
	VMStatusUnknown      = "unknown"
)

type Redis struct {
	UUID   structure.UUID `json:"UUID"`
	Status string         `json:"status"`
}

func ValidateAndNormalizeStatus(status string) string {
	if status == "" || status == "null" {
		return VMStatusUnknown
	}

	switch status {
	case VMStatusPrepareBegin, VMStatusStartBegin, VMStatusStarted, VMStatusStopped,
		VMStatusRelease, VMStatusMigrate, VMStatusRestore, VMStatusUnknown:
		return status
	default:
		return VMStatusUnknown
	}
}
