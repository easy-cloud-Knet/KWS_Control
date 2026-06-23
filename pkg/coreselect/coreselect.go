// Package coreselect는 "여러 자원(cpu/mem/disk)을 함께 고려해, 남은 자원 비율이 요청 비율과
// 가장 비슷한 코어"를 고르는 순수 선택 알고리즘이다. service/structure/redis에 의존하지 않고
// 평범한 struct만 입출력으로 받으므로 단독 테스트·수정이 쉽다.
//
// 절차: 하드 필터(가용성 AND 조건) → 코사인 유사도 점수 → 타이브레이커.
// 단위: mem/disk는 MiB, cpu는 논리 코어 수. 호출자가 단위를 일관되게 맞춘다(추가 변환 금지).
package coreselect

import "math"

// Resources는 (cpu, mem, disk) 자원량 또는 비율.
type Resources struct {
	CPU, Mem, Disk float64
}

// CoreInput은 한 코어의 가용 용량과 현재 할당량.
//   - Capacity.CPU: 오버커밋이 이미 반영된 가용 vCPU
//   - Capacity.Mem/Disk: 코어 전체 용량(MiB)
//   - Alloc: 현재 할당 합(논리 코어 수 / MiB)
type CoreInput struct {
	Capacity Resources
	Alloc    Resources
}

// Params는 선택 알고리즘 파라미터.
type Params struct {
	MemReservePct  float64 // 0..1, 메모리 여유분 비율(필터에서만 적용)
	DiskReservePct float64 // 0..1, 디스크 여유분 비율(필터에서만 적용)
	Weights        Weights // 코사인 유사도 차원별 가중치(전부 0이면 동등 가중)
}

// Weights는 코사인 유사도 각 차원의 가중치(비음수). 작을수록 해당 자원이 선택에 덜 기여한다.
// 예: Disk를 1보다 작게 두면 "disk는 (필터를 통과하는 한) cpu/mem보다 덜 중요"를 반영.
type Weights struct {
	CPU, Mem, Disk float64
}

// Choose는 하드 필터를 통과한 코어 중 코사인 유사도가 가장 높은 코어의 인덱스(cores 기준)를 반환한다.
// feasible한 코어가 없으면 -1.
// 타이브레이커: ① 코사인 최대 → ② 남은 자원 magnitude 최대(더 빈 코어 우선) → ③ 낮은 인덱스.
func Choose(req Resources, cores []CoreInput, p Params) int {
	reqVec := vec3{req.CPU, req.Mem, req.Disk}
	w := normalizeWeights(p.Weights)

	var cands []candidate
	for i := range cores {
		cp := coreCap{cores[i].Capacity.CPU, cores[i].Capacity.Mem, cores[i].Capacity.Disk}
		alloc := vec3{cores[i].Alloc.CPU, cores[i].Alloc.Mem, cores[i].Alloc.Disk}

		cpuOk, memOk, diskOk := feasible(reqVec, cp, alloc, p.MemReservePct, p.DiskReservePct)
		if !(cpuOk && memOk && diskOk) {
			continue
		}
		score, remaining := scoreCore(reqVec, cp, alloc, w)
		cands = append(cands, candidate{coreIndex: i, score: score, remaining: remaining})
	}
	if len(cands) == 0 {
		return -1
	}
	return cands[chooseBest(cands)].coreIndex
}

// normalizeWeights는 음수 가중치를 0으로 클램프하고, 전부 0(미지정)이면 동등 가중(1,1,1)으로 본다.
func normalizeWeights(w Weights) Weights {
	if w.CPU < 0 {
		w.CPU = 0
	}
	if w.Mem < 0 {
		w.Mem = 0
	}
	if w.Disk < 0 {
		w.Disk = 0
	}
	if w == (Weights{}) {
		return Weights{1, 1, 1}
	}
	return w
}

// --- 이하 순수 함수(unexported). 테스트는 같은 패키지(white-box)에서 직접 검증한다. ---

// vec3는 (cpu, mem, disk) 3차원 벡터.
type vec3 struct {
	cpu, mem, disk float64
}

func (v vec3) magnitude() float64 {
	return math.Sqrt(v.cpu*v.cpu + v.mem*v.mem + v.disk*v.disk)
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

// weightedDot/weightedMagnitude/weightedCosine은 차원별 가중 코사인 유사도. w가 작은 차원은 유사도에 덜 기여한다.
func weightedDot(a, b vec3, w Weights) float64 {
	return w.CPU*a.cpu*b.cpu + w.Mem*a.mem*b.mem + w.Disk*a.disk*b.disk
}

func weightedMagnitude(a vec3, w Weights) float64 {
	return math.Sqrt(w.CPU*a.cpu*a.cpu + w.Mem*a.mem*a.mem + w.Disk*a.disk*a.disk)
}

// weightedCosine은 가중 코사인 유사도. 크기가 0(영벡터 또는 영가중)이면 0을 반환(NaN 방지).
func weightedCosine(a, b vec3, w Weights) float64 {
	ma, mb := weightedMagnitude(a, w), weightedMagnitude(b, w)
	if ma == 0 || mb == 0 {
		return 0
	}
	return weightedDot(a, b, w) / (ma * mb)
}

// cosineSimilarity는 동등 가중(unweighted) 코사인 — weightedCosine의 특수형.
func cosineSimilarity(a, b vec3) float64 {
	return weightedCosine(a, b, Weights{1, 1, 1})
}

// ratio는 num/den. den이 0 이하이거나 num이 음수면 0(정의 불가/음수 비율 방지).
func ratio(num, den float64) float64 {
	if den <= 0 || num < 0 {
		return 0
	}
	return num / den
}

// feasible은 차원별로 req가 (용량 - 할당 - reserve) 안에 들어가는지 판정한다.
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
func scoreCore(req vec3, cap coreCap, alloc vec3, w Weights) (score float64, remaining vec3) {
	remaining = coreRemainingVector(cap, alloc)
	if remaining.cpu <= 0 || remaining.mem <= 0 || remaining.disk <= 0 {
		return -1, remaining
	}
	return weightedCosine(remaining, requestVector(req, cap), w), remaining
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
