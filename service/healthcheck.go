package service

import (
	"context"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/util"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

// StartHealthcheck는 주기적으로 모든 코어의 상태를 갱신한다(기본 30s).
// ctx.Done()이면 종료. main에서 go로 실행.
func StartHealthcheck(ctx context.Context, contextStruct *vms.ControlContext, interval time.Duration) {
	log := util.GetLogger()

	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info("healthcheck started (interval=%s)", interval, true)

	for {
		select {
		case <-ctx.Done():
			log.Info("healthcheck stopped", true)
			return
		case <-ticker.C:
			checkAllCores(ctx, contextStruct)
		}
	}
}

// checkAllCores는 코어 목록을 스냅샷한 뒤 락 없이 /getStatusHost를 호출하고,
// 결과만 Lock 하에 반영한다(CreateVM과 동일 규율: 네트워크 I/O는 락 밖, 쓰기만 락 안).
func checkAllCores(ctx context.Context, contextStruct *vms.ControlContext) {
	log := util.GetLogger()

	type snap struct {
		ip        string
		port      uint16
		cpuCached uint32
	}

	contextStruct.RLock()
	snaps := make([]snap, len(contextStruct.Cores))
	for i := range contextStruct.Cores {
		snaps[i] = snap{
			ip:        contextStruct.Cores[i].IP,
			port:      contextStruct.Cores[i].Port,
			cpuCached: contextStruct.Cores[i].CoreInfoIdx.Cpu,
		}
	}
	contextStruct.RUnlock()

	for _, s := range snaps {
		tmp := &vms.Core{IP: s.ip, Port: s.port}
		coreClient := client.NewCoreClient(tmp)

		memResp, memErr := coreClient.GetCoreMachineMemoryInfo(ctx)
		diskResp, diskErr := coreClient.GetCoreMachineDiskInfo(ctx)

		// CPU 총량은 캐시가 비었을 때(==0)만 새로 측정. 평소엔 mem/disk만 갱신.
		var newCpu uint32
		if s.cpuCached == 0 {
			if cpuResp, cpuErr := coreClient.GetCoreMachineCpuInfo(ctx); cpuErr == nil &&
				cpuResp != nil && cpuResp.Desc != nil && cpuResp.Desc.Total > 0 {
				newCpu = uint32(cpuResp.Desc.Total)
			}
		}

		// 양방향: mem/disk 모두 성공해야 alive(복귀 코어 재활성화 포함).
		// 응답 본문(Information 포인터)이 nil이면 데이터가 없는 것이므로 not-alive 처리.
		alive := memErr == nil && diskErr == nil && memResp != nil && diskResp != nil

		contextStruct.Lock()
		core := findCoreByAddr(contextStruct.Cores, s.ip, s.port)
		if core != nil {
			wasAlive := core.IsAlive
			core.IsAlive = alive
			if alive {
				core.CoreInfoIdx.Memory = uint32(memResp.Total * 1024)
				core.CoreInfoIdx.Disk = uint32(diskResp.Total * 1024)
				core.FreeMemory = uint32(memResp.Available * 1024)
				core.FreeDisk = uint32(diskResp.Free * 1024)
				if newCpu > 0 {
					core.CoreInfoIdx.Cpu = newCpu
				}
			}
			if wasAlive != alive {
				log.Info("healthcheck: core %s:%d alive %t -> %t", s.ip, s.port, wasAlive, alive, true)
			}
		}
		contextStruct.Unlock()

		if !alive {
			log.DebugWarn("healthcheck: core %s:%d unreachable (memErr=%v, diskErr=%v)", s.ip, s.port, memErr, diskErr)
		}
	}
}

func findCoreByAddr(cores []vms.Core, ip string, port uint16) *vms.Core {
	for i := range cores {
		if cores[i].IP == ip && cores[i].Port == port {
			return &cores[i]
		}
	}
	return nil
}
