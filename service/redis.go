package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"
)

func StoreVMInfoToRedis(ctx context.Context, rdb *redis.Client, vmInfo model.VMRedisInfo) error {
	log := util.GetLogger()

	key := string(vmInfo.UUID)

	jsonData, err := json.Marshal(vmInfo)
	if err != nil {
		log.Error("failed to marshal vm for redis %v", err, true)
		return fmt.Errorf("failed to marshal vm for redis: %w", err)
	}

	if err := rdb.Set(ctx, key, string(jsonData), 0).Err(); err != nil {
		log.Error("failed to store vm in redis: UUID=%s, error=%v", key, err, true)
		return fmt.Errorf("failed to store vm in redis: %w", err)
	}

	log.Info("vm info stored in redis: UUID=%s, CPU=%d, Memory=%d, Disk=%d, IP=%s, Status=%s, Time=%d",
		key, vmInfo.CPU, vmInfo.Memory, vmInfo.Disk, vmInfo.IP, vmInfo.Status, vmInfo.Time, true)

	return nil
}

func RemoveVMInfoFromRedis(ctx context.Context, rdb *redis.Client, uuid structure.UUID) error {
	log := util.GetLogger()

	key := string(uuid)

	result, err := rdb.Del(ctx, key).Result()
	if err != nil {
		log.Error("failed to delete vm from redis: UUID=%s, error=%v", key, err, true)
		return fmt.Errorf("failed to delete vm from redis: %w", err)
	}

	if result == 0 {
		log.Warn("vm info not found in redis during deletion: UUID=%s", key, true)
	} else {
		log.Info("vm info removed from redis: UUID=%s", key, true)
	}

	return nil
}

func GetVMInfoFromRedis(ctx context.Context, rdb *redis.Client, uuid structure.UUID) (model.VMRedisInfo, error) {
	log := util.GetLogger()

	key := string(uuid)
	var vmInfo model.VMRedisInfo

	jsonData, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("vm info not found in redis: UUID=%s", key, true)
			return vmInfo, fmt.Errorf("vm info not found in redis: %s", key)
		}
		log.Error("failed to get vm from redis: UUID=%s, error=%v", key, err, true)
		return vmInfo, fmt.Errorf("failed to get vm from redis: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonData), &vmInfo); err != nil {
		log.Error("failed to unmarshal vm from redis: UUID=%s, error=%v", key, err, true)
		return vmInfo, fmt.Errorf("failed to unmarshal vm from redis: %w", err)
	}

	log.DebugInfo("vm info retrieved from redis: UUID=%s", key)
	return vmInfo, nil
}

// VM status랑 time 한 번에 업뎃.. 통합 필요
func UpdateVMStatusInRedis(ctx context.Context, rdb *redis.Client, uuid structure.UUID, status string, timestamp int64) error {
	log := util.GetLogger()

	key := string(uuid)

	vmInfo, err := GetVMInfoFromRedis(ctx, rdb, uuid)
	if err != nil {
		log.Error("failed to get existing vm info from redis: UUID=%s, error=%v", key, err, true)
		return fmt.Errorf("failed to get existing vm info from redis: %w", err)
	}

	vmInfo.Status = status
	vmInfo.Time = timestamp

	jsonData, err := json.Marshal(vmInfo)
	if err != nil {
		log.Error("failed to marshal updated vm for redis: UUID=%s, error=%v", key, err, true)
		return fmt.Errorf("failed to marshal updated vm for redis: %w", err)
	}

	if err := rdb.Set(ctx, key, string(jsonData), 0).Err(); err != nil {
		log.Error("failed to update vm status in redis: UUID=%s, error=%v", key, err, true)
		return fmt.Errorf("failed to update vm status in redis: %w", err)
	}

	log.Info("vm status updated in redis: UUID=%s, Status=%s, Time=%d", key, status, timestamp, true)
	return nil
}
