package coreselect

import (
	"math"
	"testing"
)

const eps = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < eps
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b vec3
		want float64
	}{
		{"identical direction", vec3{1, 1, 1}, vec3{2, 2, 2}, 1.0},
		{"same vector", vec3{0.5, 0.3, 0.2}, vec3{0.5, 0.3, 0.2}, 1.0},
		{"orthogonal", vec3{1, 0, 0}, vec3{0, 1, 0}, 0.0},
		{"zero a", vec3{0, 0, 0}, vec3{1, 1, 1}, 0.0},
		{"zero b", vec3{1, 1, 1}, vec3{0, 0, 0}, 0.0},
		{"both zero", vec3{0, 0, 0}, vec3{0, 0, 0}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.IsNaN(got) {
				t.Fatalf("cosineSimilarity(%v,%v) = NaN, want %v", tt.a, tt.b, tt.want)
			}
			if !almostEqual(got, tt.want) {
				t.Errorf("cosineSimilarity(%v,%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestRatio(t *testing.T) {
	tests := []struct {
		name     string
		num, den float64
		want     float64
	}{
		{"normal fraction", 1, 4, 0.25},
		{"den zero", 5, 0, 0},
		{"den negative", 5, -2, 0},
		{"num negative", -3, 10, 0},
		{"both zero", 0, 0, 0},
		{"num zero", 0, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ratio(tt.num, tt.den); !almostEqual(got, tt.want) {
				t.Errorf("ratio(%v,%v) = %v, want %v", tt.num, tt.den, got, tt.want)
			}
		})
	}
}

func TestFeasibleBoundaries(t *testing.T) {
	// logical=4, overcommit=2 → cpu cap = 8
	// mem total = 1000, mem reserve = 0.1 → reserve 100
	// disk total = 2000, disk reserve = 0.25 → reserve 500
	cap := coreCap{cpu: 8, mem: 1000, disk: 2000}
	alloc := vec3{cpu: 2, mem: 200, disk: 300}
	memReserve, diskReserve := 0.1, 0.25

	// available per dim:
	//   cpu : 8 - 2                       = 6
	//   mem : 1000 - (200 + 1000*0.1)     = 700
	//   disk: 2000 - (300 + 2000*0.25)    = 1200

	// exact boundary: all pass
	if c, m, d := feasible(vec3{6, 700, 1200}, cap, alloc, memReserve, diskReserve); !(c && m && d) {
		t.Errorf("boundary req should pass, got cpu=%t mem=%t disk=%t", c, m, d)
	}

	// cpu + 1 fails, others pass
	if c, m, d := feasible(vec3{7, 700, 1200}, cap, alloc, memReserve, diskReserve); c || !m || !d {
		t.Errorf("cpu+1 should fail only cpu, got cpu=%t mem=%t disk=%t", c, m, d)
	}
	// mem + 1 fails, others pass
	if c, m, d := feasible(vec3{6, 701, 1200}, cap, alloc, memReserve, diskReserve); !c || m || !d {
		t.Errorf("mem+1 should fail only mem, got cpu=%t mem=%t disk=%t", c, m, d)
	}
	// disk + 1 fails, others pass
	if c, m, d := feasible(vec3{6, 700, 1201}, cap, alloc, memReserve, diskReserve); !c || !m || d {
		t.Errorf("disk+1 should fail only disk, got cpu=%t mem=%t disk=%t", c, m, d)
	}
}

func TestFeasibleOvercommit(t *testing.T) {
	// Without overcommit (cap.cpu = logical = 4), req 6 fails.
	// With overcommit x2 (cap.cpu = 8), req 6 passes.
	noOC := coreCap{cpu: 4, mem: 1000, disk: 1000}
	withOC := coreCap{cpu: 8, mem: 1000, disk: 1000}
	alloc := vec3{}

	if c, _, _ := feasible(vec3{6, 0, 0}, noOC, alloc, 0, 0); c {
		t.Errorf("req cpu=6 should fail without overcommit (cap=4)")
	}
	if c, _, _ := feasible(vec3{6, 0, 0}, withOC, alloc, 0, 0); !c {
		t.Errorf("req cpu=6 should pass with overcommit (cap=8)")
	}
}

func TestChooseBest(t *testing.T) {
	t.Run("empty returns -1", func(t *testing.T) {
		if got := chooseBest(nil); got != -1 {
			t.Errorf("chooseBest(nil) = %d, want -1", got)
		}
	})

	t.Run("highest score (shape) wins", func(t *testing.T) {
		cands := []candidate{
			{coreIndex: 0, score: 0.5, remaining: vec3{0.9, 0.9, 0.9}},
			{coreIndex: 1, score: 0.95, remaining: vec3{0.2, 0.2, 0.2}},
			{coreIndex: 2, score: 0.7, remaining: vec3{0.5, 0.5, 0.5}},
		}
		if got := chooseBest(cands); got != 1 {
			t.Errorf("chooseBest = %d, want 1 (highest score)", got)
		}
	})

	t.Run("guard negative excluded when feasible exists", func(t *testing.T) {
		cands := []candidate{
			{coreIndex: 0, score: -1, remaining: vec3{0, 0.5, 0.5}},
			{coreIndex: 1, score: 0.3, remaining: vec3{0.4, 0.4, 0.4}},
		}
		if got := chooseBest(cands); got != 1 {
			t.Errorf("chooseBest = %d, want 1 (positive over guard-negative)", got)
		}
	})

	t.Run("tie on score breaks by magnitude (emptier core)", func(t *testing.T) {
		cands := []candidate{
			{coreIndex: 0, score: 0.8, remaining: vec3{0.3, 0.3, 0.3}}, // smaller magnitude
			{coreIndex: 1, score: 0.8, remaining: vec3{0.6, 0.6, 0.6}}, // larger magnitude
		}
		if got := chooseBest(cands); got != 1 {
			t.Errorf("chooseBest = %d, want 1 (larger remaining magnitude)", got)
		}
	})

	t.Run("tie on score and magnitude breaks by lowest index", func(t *testing.T) {
		cands := []candidate{
			{coreIndex: 3, score: 0.8, remaining: vec3{0.5, 0.5, 0.5}},
			{coreIndex: 7, score: 0.8, remaining: vec3{0.5, 0.5, 0.5}},
		}
		if got := chooseBest(cands); got != 0 {
			t.Errorf("chooseBest = %d, want 0 (lowest index via forward scan)", got)
		}
	})
}

func TestChoose(t *testing.T) {
	p := Params{MemReservePct: 0, DiskReservePct: 0}

	t.Run("no cores returns -1", func(t *testing.T) {
		if got := Choose(Resources{1, 100, 100}, nil, p); got != -1 {
			t.Errorf("Choose(nil) = %d, want -1", got)
		}
	})

	t.Run("infeasible core filtered out", func(t *testing.T) {
		cores := []CoreInput{
			{Capacity: Resources{2, 1000, 1000}, Alloc: Resources{2, 0, 0}}, // cpu full
		}
		if got := Choose(Resources{1, 100, 100}, cores, p); got != -1 {
			t.Errorf("Choose = %d, want -1 (only core infeasible)", got)
		}
	})

	t.Run("returns index into input slice", func(t *testing.T) {
		cores := []CoreInput{
			{Capacity: Resources{2, 1000, 1000}, Alloc: Resources{2, 0, 0}},   // 0: infeasible (cpu full)
			{Capacity: Resources{8, 8000, 8000}, Alloc: Resources{0, 0, 0}},   // 1: feasible, very empty
			{Capacity: Resources{4, 4000, 4000}, Alloc: Resources{1, 500, 0}}, // 2: feasible
		}
		got := Choose(Resources{1, 500, 500}, cores, p)
		if got != 1 && got != 2 {
			t.Errorf("Choose = %d, want a feasible index (1 or 2)", got)
		}
		if got == 0 {
			t.Errorf("Choose returned infeasible core index 0")
		}
	})
}

func TestChooseWeighting(t *testing.T) {
	// 두 코어는 cpu↔disk 대칭: A는 disk가 빡빡(remaining 0.5)·cpu 여유, B는 cpu가 빡빡·disk 여유.
	// 용량이 같아 requestVector도 동일 → 동등 가중이면 점수가 정확히 타이(대칭).
	cores := []CoreInput{
		{Capacity: Resources{10, 10, 10}, Alloc: Resources{0, 0, 5}}, // 0(A): remaining {1, 1, 0.5}
		{Capacity: Resources{10, 10, 10}, Alloc: Resources{5, 0, 0}}, // 1(B): remaining {0.5, 1, 1}
	}
	req := Resources{1, 1, 1}

	// disk 비중을 낮추면 "disk가 빡빡한" A를 선호 — disk 부족은 덜 중요하다는 의도 반영.
	if got := Choose(req, cores, Params{Weights: Weights{1, 1, 0.1}}); got != 0 {
		t.Errorf("low disk weight: Choose = %d, want 0 (A)", got)
	}
	// 대칭 확인: cpu 비중을 낮추면 "cpu가 빡빡한" B를 선호.
	if got := Choose(req, cores, Params{Weights: Weights{0.1, 1, 1}}); got != 1 {
		t.Errorf("low cpu weight: Choose = %d, want 1 (B)", got)
	}
}
