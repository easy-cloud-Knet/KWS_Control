package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/easy-cloud-Knet/KWS_Control/request"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/redis/go-redis/v9"
)

var (
	ErrVMNotFoundInMetadata = errors.New("vm metadata not found")
	ErrVMNotOrphaned        = errors.New("vm is not orphaned")
	ErrVMVerificationFailed = errors.New("failed to verify vm orphan state")
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

type OrphanedDeleteSuccess struct {
	UUID    structure.UUID `json:"uuid"`
	Deleted DeleteTargets  `json:"deleted"`
}

type OrphanedDeleteFailure struct {
	UUID        structure.UUID `json:"uuid"`
	Reason      string         `json:"reason"`
	ShouldRetry bool           `json:"should_retry"`
}

type OrphanedDeleteBatchResult struct {
	Deleted      []OrphanedDeleteSuccess `json:"deleted"`
	Failed       []OrphanedDeleteFailure `json:"failed"`
	TotalTarget  int                     `json:"total_target"`
	TotalDeleted int                     `json:"total_deleted"`
	TotalFailed  int                     `json:"total_failed"`
}

type orphanedMetadataSnapshot struct {
	vmInfo      *structure.VMInfo
	coreIdx     int
	redisValue  string
	redisExists bool
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

func DeleteAllOrphanedVMData(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client) (OrphanedDeleteBatchResult, error) {
	scanResult, err := GetOrphanedVMs(ctx, contextStruct, rdb)
	if err != nil {
		return OrphanedDeleteBatchResult{}, err
	}

	result := OrphanedDeleteBatchResult{
		Deleted:     make([]OrphanedDeleteSuccess, 0),
		Failed:      make([]OrphanedDeleteFailure, 0),
		TotalTarget: len(scanResult.Orphaned),
	}

	for _, orphan := range scanResult.Orphaned {
		deleteTargets := orphan.Delete
		if purgeErr := purgeOrphanedMetadata(ctx, contextStruct, rdb, orphan.UUID); purgeErr != nil {
			result.Failed = append(result.Failed, OrphanedDeleteFailure{
				UUID:        orphan.UUID,
				Reason:      purgeErr.Error(),
				ShouldRetry: true,
			})
			continue
		}
		result.Deleted = append(result.Deleted, OrphanedDeleteSuccess{
			UUID:    orphan.UUID,
			Deleted: deleteTargets,
		})
	}

	result.TotalDeleted = len(result.Deleted)
	result.TotalFailed = len(result.Failed)
	return result, nil
}

func DeleteOrphanedVMDataByUUID(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client, uuid structure.UUID) (OrphanedDeleteSuccess, error) {
	if _, err := contextStruct.GetInstance(uuid); err != nil {
		return OrphanedDeleteSuccess{}, fmt.Errorf("%w: %s", ErrVMNotFoundInMetadata, uuid)
	}

	coreIdx, err := contextStruct.GetInstanceLocation(uuid)
	if err != nil {
		return OrphanedDeleteSuccess{}, fmt.Errorf("%w: %s", ErrVMNotFoundInMetadata, uuid)
	}
	if coreIdx < 0 || coreIdx >= len(contextStruct.Cores) {
		return OrphanedDeleteSuccess{}, fmt.Errorf("%w: invalid core index %d for uuid %s", ErrVMVerificationFailed, coreIdx, uuid)
	}

	core := &contextStruct.Cores[coreIdx]
	client := request.NewCoreClient(core)
	_, checkErr := client.GetVMCpuInfo(ctx, uuid)
	if checkErr == nil {
		return OrphanedDeleteSuccess{}, fmt.Errorf("%w: uuid %s still exists on core %s:%d", ErrVMNotOrphaned, uuid, core.IP, core.Port)
	}
	if !isVmNotFoundError(checkErr) {
		return OrphanedDeleteSuccess{}, fmt.Errorf("%w: %v", ErrVMVerificationFailed, checkErr)
	}

	redisKey := string(uuid)
	redisExists := false
	if rdb != nil {
		existsCount, redisErr := rdb.Exists(ctx, redisKey).Result()
		if redisErr == nil {
			redisExists = existsCount > 0
		}
	}

	deleteTargets := DeleteTargets{
		InstInfoUUID:            uuid,
		InstLocUUID:             uuid,
		InstLocCoreIndex:        coreIdx,
		RedisKey:                redisKey,
		RedisExists:             redisExists,
		GuacamoleEntityName:     string(uuid),
		GuacamoleConnectionName: fmt.Sprintf("%s-ssh", uuid),
	}

	if purgeErr := purgeOrphanedMetadata(ctx, contextStruct, rdb, uuid); purgeErr != nil {
		return OrphanedDeleteSuccess{}, purgeErr
	}

	return OrphanedDeleteSuccess{
		UUID:    uuid,
		Deleted: deleteTargets,
	}, nil
}

func purgeOrphanedMetadata(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client, uuid structure.UUID) error {
	snapshot, err := snapshotOrphanedMetadata(ctx, contextStruct, rdb, uuid)
	if err != nil {
		return fmt.Errorf("failed to snapshot metadata for %s: %w", uuid, err)
	}

	if err := contextStruct.DeleteInstance(uuid); err != nil {
		return fmt.Errorf("failed to delete db metadata for %s: %w", uuid, err)
	}
	if rdb != nil {
		if err := RemoveVMInfoFromRedis(ctx, rdb, uuid); err != nil {
			compensationErr := restoreOrphanedMetadata(ctx, contextStruct, rdb, snapshot)
			if compensationErr != nil {
				return fmt.Errorf("failed to delete redis metadata for %s: %w (compensation failed: %v)", uuid, err, compensationErr)
			}
			return fmt.Errorf("failed to delete redis metadata for %s: %w (compensation applied)", uuid, err)
		}
	}
	if err := CleanupGuacamoleConfig(string(uuid), contextStruct.GuacDB); err != nil {
		compensationErr := restoreOrphanedMetadata(ctx, contextStruct, rdb, snapshot)
		if compensationErr != nil {
			return fmt.Errorf("failed to delete guacamole metadata for %s: %w (compensation failed: %v)", uuid, err, compensationErr)
		}
		return fmt.Errorf("failed to delete guacamole metadata for %s: %w (compensation applied)", uuid, err)
	}

	delete(contextStruct.VMLocation, uuid)
	for i := range contextStruct.Cores {
		delete(contextStruct.Cores[i].VMInfoIdx, uuid)
	}
	for i := len(contextStruct.AliveVM) - 1; i >= 0; i-- {
		if contextStruct.AliveVM[i].UUID == uuid {
			contextStruct.AliveVM = slices.Delete(contextStruct.AliveVM, i, i+1)
		}
	}

	return nil
}

func snapshotOrphanedMetadata(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client, uuid structure.UUID) (orphanedMetadataSnapshot, error) {
	vmInfo, err := contextStruct.GetInstance(uuid)
	if err != nil {
		return orphanedMetadataSnapshot{}, err
	}

	coreIdx, err := contextStruct.GetInstanceLocation(uuid)
	if err != nil {
		return orphanedMetadataSnapshot{}, err
	}

	snapshot := orphanedMetadataSnapshot{
		vmInfo:  vmInfo,
		coreIdx: coreIdx,
	}

	if rdb != nil {
		redisValue, redisErr := rdb.Get(ctx, string(uuid)).Result()
		if redisErr == nil {
			snapshot.redisValue = redisValue
			snapshot.redisExists = true
		}
	}

	return snapshot, nil
}

func restoreOrphanedMetadata(ctx context.Context, contextStruct *structure.ControlContext, rdb *redis.Client, snapshot orphanedMetadataSnapshot) error {
	if snapshot.vmInfo == nil {
		return errors.New("snapshot vm info is nil")
	}

	if err := contextStruct.AddInstance(snapshot.vmInfo, snapshot.coreIdx); err != nil {
		return fmt.Errorf("failed to restore db metadata for %s: %w", snapshot.vmInfo.UUID, err)
	}

	if rdb != nil && snapshot.redisExists {
		if err := rdb.Set(ctx, string(snapshot.vmInfo.UUID), snapshot.redisValue, 0).Err(); err != nil {
			return fmt.Errorf("failed to restore redis metadata for %s: %w", snapshot.vmInfo.UUID, err)
		}
	}

	return nil
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
