package service

import (
	"context"
	"fmt"
	"math"

	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

// 코어 선택 = 하드 필터(가용성 AND 조건) → 코사인 유사도 가중치.
//
// 단위: mem/disk는 전 구간 MiB, cpu는 논리 코어 수. 추가 변환 금지.

// vec3는 (cpu, mem, disk) 3차원 벡터.
type vec3 struct {
	cpu, mem, disk float64
}

func (v vec3) magnitude() float64 {
	return math.Sqrt(v.cpu*v.cpu + v.mem*v.mem + v.disk*v.disk)
}

func dot(a, b vec3) float64 {
	return a.cpu*b.cpu + a.mem*b.mem + a.disk*b.disk
}

// coreCap은 한 코어의 차원별 용량.
//   - cpu: logical_cpu * cpu_overcommit (오버커밋이 이미 반영된 가용 vCPU)
//   - mem/disk: 코어의 전체 용량(MiB)
type coreCap struct {
	cpu, mem, disk float64
}

// candidate는 하드 필터를 통과한 후보 코어와 점수.
type candidate struct {
	coreIndex int
	score     float64
	remaining vec3 // 남은 자원 비율 벡터(코사인은 크기 무시 → 타이브레이커로 magnitude 사용)
}

// cosineSimilarity는 두 벡터의 코사인 유사도. 영벡터(zero-magnitude)는 0을 반환(NaN 방지).
func cosineSimilarity(a, b vec3) float64 {
	ma, mb := a.magnitude(), b.magnitude()
	if ma == 0 || mb == 0 {
		return 0
	}
	return dot(a, b) / (ma * mb)
}

// ratio는 num/den. den이 0 이하이거나 num이 음수면 0(정의 불가/음수 비율 방지).
func ratio(num, den float64) float64 {
	if den <= 0 || num < 0 {
		return 0
	}
	return num / den
}

// feasible은 차원별로 req가 (용량 - 할당 - reserve) 안에 들어가는지 판정한다.
// 하드 필터 정의와 정확히 일치:
//   - cpu : req.cpu  ≤ cap.cpu  - alloc.cpu                       (cap.cpu = logical*overcommit)
//   - mem : req.mem  ≤ cap.mem  - (alloc.mem  + cap.mem*memReservePct)
//   - disk: req.disk ≤ cap.disk - (alloc.disk + cap.disk*diskReservePct)
func feasible(req vec3, cap coreCap, alloc vec3, memReservePct, diskReservePct float64) (cpuOk, memOk, diskOk bool) {
	cpuOk = req.cpu <= cap.cpu-alloc.cpu
	memOk = req.mem <= cap.mem-(alloc.mem+cap.mem*memReservePct)
	diskOk = req.disk <= cap.disk-(alloc.disk+cap.disk*diskReservePct)
	return
}

// coreRemainingVector는 차원별 남은 비율 ((capacity - used)/capacity). reserve는 미반영(필터에서만 적용).
func coreRemainingVector(cap coreCap, alloc vec3) vec3 {
	return vec3{
		cpu:  ratio(cap.cpu-alloc.cpu, cap.cpu),
		mem:  ratio(cap.mem-alloc.mem, cap.mem),
		disk: ratio(cap.disk-alloc.disk, cap.disk),
	}
}

// requestVector는 해당 코어 기준 요청 비율 (req.cpu/cap.cpu, req.mem/cap.mem, req.disk/cap.disk).
func requestVector(req vec3, cap coreCap) vec3 {
	return vec3{
		cpu:  ratio(req.cpu, cap.cpu),
		mem:  ratio(req.mem, cap.mem),
		disk: ratio(req.disk, cap.disk),
	}
}

// scoreCore는 코어의 점수와 남은 비율 벡터를 계산한다.
// 어느 차원이든 남은 값이 0 이하이면 강한 음수 점수(가드) — 하드 필터의 이중 안전장치.
func scoreCore(req vec3, cap coreCap, alloc vec3) (score float64, remaining vec3) {
	remaining = coreRemainingVector(cap, alloc)
	if remaining.cpu <= 0 || remaining.mem <= 0 || remaining.disk <= 0 {
		return -1, remaining
	}
	return cosineSimilarity(remaining, requestVector(req, cap)), remaining
}

// chooseBest는 후보 중 최적 인덱스를 결정적으로 고른다. 빈 슬라이스면 -1.
// 타이브레이커: ① 코사인 최대 → ② 남은 자원 magnitude 최대(더 빈 코어 우선)
// → ③ 낮은 인덱스(전방 스캔 + strict `>`로 자연 보장).
func chooseBest(cands []candidate) int {
	best := -1
	for i := range cands {
		switch {
		case best == -1:
			best = i
		case cands[i].score > cands[best].score:
			best = i
		case cands[i].score == cands[best].score &&
			cands[i].remaining.magnitude() > cands[best].remaining.magnitude():
			best = i
		}
	}
	return best
}

// SelectCore는 살아있는 코어 중 하드 필터를 통과한 후보를 코사인 유사도로 평가해 최적 코어를 고른다.
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
	memReserve := contextStruct.Config.MemReservePct
	diskReserve := contextStruct.Config.DiskReservePct

	reqVec := vec3{
		cpu:  float64(req.CPU),
		mem:  float64(req.Memory),
		disk: float64(req.Disk),
	}

	var cands []candidate
	aliveCount := 0

	for i := range contextStruct.Cores {
		core := &contextStruct.Cores[i]
		if !core.IsAlive {
			continue
		}
		aliveCount++

		cap := coreCap{
			cpu:  float64(core.CoreInfoIdx.Cpu) * overcommit,
			mem:  float64(core.CoreInfoIdx.Memory),
			disk: float64(core.CoreInfoIdx.Disk),
		}

		alloc, err := GetCoreAlloc(ctx, rdb, core.IP, core.Port)
		if err != nil {
			log.Warn("SelectCore: failed to read alloc for core %s:%d, skipping: %v", core.IP, core.Port, err)
			continue
		}
		allocVec := vec3{
			cpu:  float64(alloc.CPU),
			mem:  float64(alloc.Mem),
			disk: float64(alloc.Disk),
		}

		cpuOk, memOk, diskOk := feasible(reqVec, cap, allocVec, memReserve, diskReserve)
		if !(cpuOk && memOk && diskOk) {
			log.DebugInfo("core %s:%d rejected by filter: cpu=%t mem=%t disk=%t (alloc cpu=%d mem=%d disk=%d)",
				core.IP, core.Port, cpuOk, memOk, diskOk, alloc.CPU, alloc.Mem, alloc.Disk)
			continue
		}

		score, remaining := scoreCore(reqVec, cap, allocVec)
		cands = append(cands, candidate{coreIndex: i, score: score, remaining: remaining})
		log.DebugInfo("core %s:%d candidate: score=%.4f remaining=(%.3f,%.3f,%.3f)",
			core.IP, core.Port, score, remaining.cpu, remaining.mem, remaining.disk)
	}

	if len(cands) == 0 {
		return nil, -1, fmt.Errorf("SelectCore: no feasible core (alive=%d, req cpu=%d mem=%d disk=%d)",
			aliveCount, req.CPU, req.Memory, req.Disk)
	}

	best := chooseBest(cands)
	chosen := cands[best]
	core := &contextStruct.Cores[chosen.coreIndex]
	log.Info("SelectCore: chose core %s:%d (score=%.4f) for req cpu=%d mem=%d disk=%d",
		core.IP, core.Port, chosen.score, req.CPU, req.Memory, req.Disk, true)
	return core, chosen.coreIndex, nil
}
