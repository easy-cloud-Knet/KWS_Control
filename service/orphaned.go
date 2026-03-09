package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/easy-cloud-Knet/KWS_Control/request"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/redis/go-redis/v9"
)

type DeleteTargets struct {
	InstInfoUUID            structure.UUID `json:"inst_info_uuid"`
	InstLocUUID             structure.UUID `json:"inst_loc_uuid"`
	InstLocCoreIndex        int            `json:"inst_loc_core_index"`
	RedisKey                string         `json:"redis_key"`
	RedisExists             bool           `json:"redis_exists"`
	GuacamoleEntityName     string         `json:"guacamole_entity_name"`
	GuacamoleConnectionName string         `json:"guacamole_connection_name"`
}

type OrphanedVMInfo struct {
	UUID         structure.UUID `json:"uuid"`
	CoreIP       string         `json:"core_ip"`
	CorePort     uint16         `json:"core_port"`
	CoreEndpoint string         `json:"core_endpoint"`
	VMIP         string         `json:"vm_ip"`
	Delete       DeleteTargets  `json:"delete_targets"`
}

type OrphanedVMCheckFailure struct {
	UUID        structure.UUID `json:"uuid"`
	CoreIP      string         `json:"core_ip"`
	CorePort    uint16         `json:"core_port"`
	Reason      string         `json:"reason"`
	ShouldRetry bool           `json:"should_retry"`
}

type OrphanedVMScanResult struct {
	Orphaned            []OrphanedVMInfo         `json:"orphaned"`
	VerificationFailed  []OrphanedVMCheckFailure `json:"verification_failed"`
	TotalChecked        int                      `json:"total_checked"`
	TotalOrphaned       int                      `json:"total_orphaned"`
	TotalFailedToVerify int                      `json:"total_failed_to_verify"`
}

func GetOrphanedVMs(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client) (OrphanedVMScanResult, error) {
	vmInfoList, coreIdxList, err := contextStruct.GetAllInstanceInfo()
	if err != nil {
		return OrphanedVMScanResult{}, fmt.Errorf("failed to read instance list: %w", err)
	}

	result := OrphanedVMScanResult{
		Orphaned:           make([]OrphanedVMInfo, 0),
		VerificationFailed: make([]OrphanedVMCheckFailure, 0),
		TotalChecked:       len(vmInfoList),
	}

	for i := range vmInfoList {
		vmInfo := vmInfoList[i]
		coreIdx := coreIdxList[i]

		if coreIdx < 0 || coreIdx >= len(contextStruct.Cores) {
			result.VerificationFailed = append(result.VerificationFailed, OrphanedVMCheckFailure{
				UUID:        vmInfo.UUID,
				Reason:      fmt.Sprintf("invalid core index in inst_loc: %d", coreIdx),
				ShouldRetry: false,
			})
			continue
		}

		core := &contextStruct.Cores[coreIdx]
		client := request.NewCoreClient(core)
		_, checkErr := client.GetVMCpuInfo(ctx, vmInfo.UUID)

		if checkErr == nil {
			continue
		}

		if !isVmNotFoundError(checkErr) {
			result.VerificationFailed = append(result.VerificationFailed, OrphanedVMCheckFailure{
				UUID:        vmInfo.UUID,
				CoreIP:      core.IP,
				CorePort:    core.Port,
				Reason:      checkErr.Error(),
				ShouldRetry: true,
			})
			continue
		}

		redisKey := string(vmInfo.UUID)
		redisExists := false
		if rdb != nil {
			existsCount, redisErr := rdb.Exists(ctx, redisKey).Result()
			if redisErr == nil {
				redisExists = existsCount > 0
			}
		}

		result.Orphaned = append(result.Orphaned, OrphanedVMInfo{
			UUID:         vmInfo.UUID,
			CoreIP:       core.IP,
			CorePort:     core.Port,
			CoreEndpoint: fmt.Sprintf("%s:%d", core.IP, core.Port),
			VMIP:         vmInfo.IP_VM,
			Delete: DeleteTargets{
				InstInfoUUID:            vmInfo.UUID,
				InstLocUUID:             vmInfo.UUID,
				InstLocCoreIndex:        coreIdx,
				RedisKey:                redisKey,
				RedisExists:             redisExists,
				GuacamoleEntityName:     string(vmInfo.UUID),
				GuacamoleConnectionName: fmt.Sprintf("%s-ssh", vmInfo.UUID),
			},
		})
	}

	result.TotalOrphaned = len(result.Orphaned)
	result.TotalFailedToVerify = len(result.VerificationFailed)
	return result, nil
}

func isVmNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status code: 404") ||
		strings.Contains(msg, "domain not found") ||
		strings.Contains(msg, "no such domain") ||
		strings.Contains(msg, "error searching domain")
}
