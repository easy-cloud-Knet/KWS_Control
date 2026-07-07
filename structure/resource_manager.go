package structure

import (
	"math"
	"slices"
	"sync"
)

// ResourceManager는 코어/VM의 런타임 인메모리 상태를 관리
// ControlContext에서 뮤텍스와 상태 필드를 분리
type ResourceManager struct {
	mu         sync.RWMutex
	Cores      []Core         // 모든 코어를 리스트
	AliveVM    []*VMInfo      // 현재 가동중인 VM의 정보
	VMLocation map[UUID]*Core // UUID 기반 VM  코어 위치 확인
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		VMLocation: make(map[UUID]*Core),
	}
}

func (rm *ResourceManager) Lock()    { rm.mu.Lock() }
func (rm *ResourceManager) Unlock()  { rm.mu.Unlock() }
func (rm *ResourceManager) RLock()   { rm.mu.RLock() }
func (rm *ResourceManager) RUnlock() { rm.mu.RUnlock() }

// HardwareRequirement는 코어 선택 시 필요한 자원 요구량 구조체
type HardwareRequirement struct {
	Memory uint32 // MiB
	CPU    uint32 // logical cores
	Disk   uint32 // MiB
}

// CoreSelectionResult는 SelectCore의 반환값으로 진단 정보를 내포
type CoreSelectionResult struct {
	Core       *Core
	Index      int
	AliveCount int
	TotalCores int
}

// 분산 기본 가중치
// 자원 우선순위 - 메모리 > 디스크 > CPU.
// 현재는 하드코딩, 나중에 백엔드? 아니면 별도의 관리자 페이지 등에서 데이터페칭하는 방식도 괜찮을 거 같음.
const (
	loadWeightMemory = 0.6  
	loadWeightDisk   = 0.3
	loadWeightCPU    = 0.1
	loadBalanceGain  = 0.5  // 자원 간 사용률 분산 페널티 강도
	saturationBase   = 1.01 // 비선형 페널티 기준, saturationPenalty()에서 사용.
)

// utilRatio는 VM 배치 후 해당 자원의 사용률 [0,1]을 반환.
// total==0(정보 없음/불량 코어)이면 1(포화 취급 → 회피).
func utilRatio(total, free, req uint32) float64 {
	if total == 0 {
		return 1
	}
	u := float64(total-free+req) / float64(total) // 호출부에서 free>=req, free<=total 보장
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return u
}

// 100%에 가까울수록 부하 가중치 급격하게 커짐.
// U=0→~0.99, 0.8→~4.8, 0.9→~9.1, 0.99→50, 1.0→100.
func saturationPenalty(u float64) float64 {
	return 1.0 / (saturationBase - u)
}

// 코어의 부하 점수 반환 (낮을수록 우수).
// 가중 포화 페널티(자원 우선순위 + 포화 회피) + 다차원 균형(분산) 페널티로 구성.
func loadScore(c *Core, req HardwareRequirement) float64 {
	uMem := utilRatio(c.CoreInfoIdx.Memory, c.FreeMemory, req.Memory)
	uDisk := utilRatio(c.CoreInfoIdx.Disk, c.FreeDisk, req.Disk)
	uCPU := utilRatio(c.CoreInfoIdx.Cpu, c.FreeCPU, req.CPU)

	weighted := loadWeightMemory*saturationPenalty(uMem) +
		loadWeightDisk*saturationPenalty(uDisk) +
		loadWeightCPU*saturationPenalty(uCPU)

	// 다차원 균형: 자원 간 사용률 분산이 클수록(고립 자원) 페널티 → stranded resource 방지
	mean := (uMem + uDisk + uCPU) / 3
	variance := ((uMem-mean)*(uMem-mean) +
		(uDisk-mean)*(uDisk-mean) +
		(uCPU-mean)*(uCPU-mean)) / 3

	return weighted + loadBalanceGain*variance
}

// SelectCore는 요청 자원을 만족하는 살아있는 코어 중 loadScore가 가장 낮은 코어를 선택.
// 적합한 코어가 없으면 Core==nil로 반환하며, 진단 로그를 위한 카운트 정보를 함께 제공
func (rm *ResourceManager) SelectCore(req HardwareRequirement) CoreSelectionResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := CoreSelectionResult{
		Index:      -1,
		TotalCores: len(rm.Cores),
	}
	bestScore := math.MaxFloat64

	for i := range rm.Cores {
		core := &rm.Cores[i]
		if !core.IsAlive {
			continue
		}
		result.AliveCount++

		if core.FreeMemory < req.Memory || core.FreeCPU < req.CPU || core.FreeDisk < req.Disk {
			continue
		}
		if s := loadScore(core, req); s < bestScore {
			bestScore = s
			result.Core = core
			result.Index = i
		}
	}
	return result
}

// AllocateResources는 코어의 VMInfoIdx 맵에 VM을 등록하고 Free* 필드를 차감
func (rm *ResourceManager) AllocateResources(core *Core, uuid UUID, vm *VMInfo, req HardwareRequirement) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if core.VMInfoIdx == nil {
		core.VMInfoIdx = make(map[UUID]*VMInfo)
	}
	core.VMInfoIdx[uuid] = vm
	core.FreeMemory -= req.Memory
	core.FreeCPU -= req.CPU
	core.FreeDisk -= req.Disk
}

// DeallocateResources는 AllocateResources의 역연산
func (rm *ResourceManager) DeallocateResources(core *Core, uuid UUID, req HardwareRequirement) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(core.VMInfoIdx, uuid)
	core.FreeMemory += req.Memory
	core.FreeCPU += req.CPU
	core.FreeDisk += req.Disk
}

// RegisterVM은 VMLocation 맵과 AliveVM 슬라이스에 VM을 동시에 등록
func (rm *ResourceManager) RegisterVM(uuid UUID, core *Core, vm *VMInfo) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.VMLocation[uuid] = core
	rm.AliveVM = append(rm.AliveVM, vm)
}

// UnregisterAlive는 AliveVM 슬라이스에서 해당 UUID를 제거
func (rm *ResourceManager) UnregisterAlive(uuid UUID) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, vm := range rm.AliveVM {
		if vm.UUID == uuid {
			rm.AliveVM = slices.Delete(rm.AliveVM, i, i+1)
			return true
		}
	}
	return false
}
