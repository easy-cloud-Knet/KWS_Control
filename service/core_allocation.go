package service

import (
	"context"
	"fmt"

	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/pkg/coreselect"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

// 코어 선택 = 하드 필터(가용성 AND 조건) → 코사인 유사도. 순수 선택 로직은 pkg/coreselect에 있다.
// SelectCore는 그 위의 어댑터로, ControlContext.Cores 순회와 alloc-Redis I/O만 담당한다.
//
// 단위: mem/disk는 전 구간 MiB, cpu는 논리 코어 수. 추가 변환 금지.

// SelectCore는 살아있는 코어 중 하드 필터를 통과한 후보를 코사인 유사도(coreselect.Choose)로 평가해 최적 코어를 고른다.
//
// 동시성: 호출자는 contextStruct.Lock()을 SelectCore + 예약(IncrCoreAlloc) 구간 전체에 유지해야 한다.
// SelectCore 자체는 contextStruct.Cores를 락 없이 읽으므로(읽기 전용) 호출자 락에 의존한다.
func SelectCore(ctx context.Context, req model.HardwareInfo,
	contextStruct *vms.ControlContext, rdb *redis.Client) (*vms.Core, int, error) {
	log := util.GetLogger()

	overcommit := contextStruct.Config.CpuOvercommit
	if overcommit <= 0 {
		overcommit = 1.0
	}
	params := coreselect.Params{
		MemReservePct:  contextStruct.Config.MemReservePct,
		DiskReservePct: contextStruct.Config.DiskReservePct,
	}

	reqRes := coreselect.Resources{
		CPU:  float64(req.CPU),
		Mem:  float64(req.Memory),
		Disk: float64(req.Disk),
	}

	// 살아있고 alloc-Redis를 읽을 수 있는 코어만 후보로 — I/O와 컨텍스트 의존은 여기서 처리하고
	// 순수 선택 로직은 coreselect.Choose에 위임한다. realIdx로 후보 인덱스→실제 코어 인덱스를 복원.
	var inputs []coreselect.CoreInput
	var realIdx []int
	aliveCount := 0
	for i := range contextStruct.Cores {
		core := &contextStruct.Cores[i]
		if !core.IsAlive {
			continue
		}
		aliveCount++

		alloc, err := GetCoreAlloc(ctx, rdb, core.IP, core.Port)
		if err != nil {
			log.Warn("SelectCore: failed to read alloc for core %s:%d, skipping: %v", core.IP, core.Port, err)
			continue
		}
		inputs = append(inputs, coreselect.CoreInput{
			Capacity: coreselect.Resources{
				CPU:  float64(core.CoreInfoIdx.Cpu) * overcommit,
				Mem:  float64(core.CoreInfoIdx.Memory),
				Disk: float64(core.CoreInfoIdx.Disk),
			},
			Alloc: coreselect.Resources{
				CPU:  float64(alloc.CPU),
				Mem:  float64(alloc.Mem),
				Disk: float64(alloc.Disk),
			},
		})
		realIdx = append(realIdx, i)
	}

	best := coreselect.Choose(reqRes, inputs, params)
	if best < 0 {
		return nil, -1, fmt.Errorf("SelectCore: no feasible core (alive=%d, req cpu=%d mem=%d disk=%d)",
			aliveCount, req.CPU, req.Memory, req.Disk)
	}

	chosenCoreIdx := realIdx[best]
	core := &contextStruct.Cores[chosenCoreIdx]
	log.Info("SelectCore: chose core %s:%d for req cpu=%d mem=%d disk=%d",
		core.IP, core.Port, req.CPU, req.Memory, req.Disk, true)
	return core, chosenCoreIdx, nil
}
