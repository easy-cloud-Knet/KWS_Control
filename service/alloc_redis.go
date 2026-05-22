package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

// 코어별 할당 집계는 기존 Redis(VM status와 동일 인스턴스)에 코어당 하나의 HASH로 저장한다.
// VM status 키(=UUID 문자열)와 네임스페이스가 겹치지 않도록 core: 접두어를 쓴다.
//   key   = core:{ip}:{port}:alloc
//   field = cpu | mem | disk   (mem/disk는 MiB, cpu는 논리 코어 수)
//
// 이 값이 자원 회계의 단일 진실 소스다(인메모리 Free*는 표시용 캐시로 격하).

// CoreAlloc은 한 코어에 현재 할당된 자원 합계.
type CoreAlloc struct {
	CPU  int64
	Mem  int64
	Disk int64
}

func coreAllocKey(ip string, port uint16) string {
	return fmt.Sprintf("core:%s:%d:alloc", ip, port)
}

func parseAllocField(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// GetCoreAlloc은 코어의 현재 할당 합계를 읽는다. 키/필드가 없으면 0.
func GetCoreAlloc(ctx context.Context, rdb *redis.Client, ip string, port uint16) (CoreAlloc, error) {
	res, err := rdb.HGetAll(ctx, coreAllocKey(ip, port)).Result()
	if err != nil {
		return CoreAlloc{}, fmt.Errorf("GetCoreAlloc %s:%d: %w", ip, port, err)
	}
	return CoreAlloc{
		CPU:  parseAllocField(res["cpu"]),
		Mem:  parseAllocField(res["mem"]),
		Disk: parseAllocField(res["disk"]),
	}, nil
}

// IncrCoreAlloc은 코어 할당을 차원별로 가감한다(예약 +, 회수/롤백 -).
// 3개 HIncrBy를 TxPipeline으로 묶어 한 차원만 반영되는 일을 막는다.
// (read-then-decide의 직렬화는 호출자의 contextStruct.Lock()이 담당 — alloc_redis.go 주석 참고.)
func IncrCoreAlloc(ctx context.Context, rdb *redis.Client, ip string, port uint16, dCPU, dMem, dDisk int64) error {
	key := coreAllocKey(ip, port)
	pipe := rdb.TxPipeline()
	pipe.HIncrBy(ctx, key, "cpu", dCPU)
	pipe.HIncrBy(ctx, key, "mem", dMem)
	pipe.HIncrBy(ctx, key, "disk", dDisk)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("IncrCoreAlloc %s:%d: %w", ip, port, err)
	}
	return nil
}

// SetCoreAlloc은 코어 할당을 절댓값으로 덮어쓴다(rebuild 전용).
func SetCoreAlloc(ctx context.Context, rdb *redis.Client, ip string, port uint16, cpu, mem, disk int64) error {
	key := coreAllocKey(ip, port)
	if err := rdb.HSet(ctx, key, "cpu", cpu, "mem", mem, "disk", disk).Err(); err != nil {
		return fmt.Errorf("SetCoreAlloc %s:%d: %w", ip, port, err)
	}
	return nil
}

// RebuildCoreAllocFromDB는 DB의 인스턴스 합계로 alloc-Redis를 재구성한다(시작 시 1회, 멱등).
// inst_loc.core(코어 인덱스)별 합계를 구해 IP:Port 키로 매핑하고, 결과에 없는 코어는 0으로
// 초기화해 이전 실행에서 남은 stale 값을 제거한다.
func RebuildCoreAllocFromDB(ctx context.Context, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()

	rows, err := contextStruct.DB.QueryContext(ctx,
		"SELECT loc.core, COALESCE(SUM(info.inst_vcpu),0), COALESCE(SUM(info.inst_mem),0), COALESCE(SUM(info.inst_disk),0) "+
			"FROM inst_loc loc JOIN inst_info info ON loc.uuid = info.uuid GROUP BY loc.core")
	if err != nil {
		return fmt.Errorf("RebuildCoreAllocFromDB: query failed: %w", err)
	}
	defer rows.Close()

	sums := make(map[int]CoreAlloc)
	for rows.Next() {
		var coreIdx int
		var a CoreAlloc
		if err := rows.Scan(&coreIdx, &a.CPU, &a.Mem, &a.Disk); err != nil {
			return fmt.Errorf("RebuildCoreAllocFromDB: scan failed: %w", err)
		}
		sums[coreIdx] = a
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("RebuildCoreAllocFromDB: rows error: %w", err)
	}

	// 코어 목록 스냅샷(idx → IP:Port). 시작 시 1회 호출이지만 healthcheck와의 경합 대비 RLock.
	contextStruct.RLock()
	type coreAddr struct {
		ip   string
		port uint16
	}
	addrs := make([]coreAddr, len(contextStruct.Cores))
	for i := range contextStruct.Cores {
		addrs[i] = coreAddr{ip: contextStruct.Cores[i].IP, port: contextStruct.Cores[i].Port}
	}
	contextStruct.RUnlock()

	for i, addr := range addrs {
		a := sums[i] // 합계에 없는 코어는 zero value → 0으로 초기화(stale 제거)
		if err := SetCoreAlloc(ctx, rdb, addr.ip, addr.port, a.CPU, a.Mem, a.Disk); err != nil {
			return fmt.Errorf("RebuildCoreAllocFromDB: %w", err)
		}
		log.Info("rebuilt alloc for core %s:%d -> cpu=%d mem=%d disk=%d", addr.ip, addr.port, a.CPU, a.Mem, a.Disk, true)
	}

	// DB의 core 인덱스가 현재 코어 목록 범위를 벗어나는 경우 경고(설정 변경 등으로 매핑 불가).
	for idx := range sums {
		if idx < 0 || idx >= len(addrs) {
			log.Warn("RebuildCoreAllocFromDB: DB core index %d has no matching core, alloc ignored", idx, true)
		}
	}

	return nil
}
