package startup

import (
	"context"
	"fmt"
	"os"

	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"
)

// InitializeRedis Redis 클라이언트를 초기화하고 연결을 테스트합니다
func InitializeRedis(ctx context.Context) (*redis.Client, error) {
	log := util.GetLogger()

	REDIS_HOST := os.Getenv("REDIS_HOST")
	if REDIS_HOST == "" {
		REDIS_HOST = "localhost:6379"
	}

	// Redis 클라이언트 생성
	rdb := redis.NewClient(&redis.Options{
		Addr: REDIS_HOST,
	})
	// 연결 테스트
	err := rdb.Set(ctx, "hello", "world", 0).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set test key in Redis: %w %s ", err, REDIS_HOST)
	}

	val, err := rdb.Get(ctx, "hello").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to set test key in Redis: %w %s ", err, REDIS_HOST)
	}

	log.Info("Redis connection test successful: hello=%s", val, true)
	return rdb, nil
}
