package structure

type UUID string

func (u UUID) String() string {
	return string(u)
}

type Config struct {
	VmInternalSubnets []string `yaml:"vm_internal_subnets"`
	GuacBaseURL       string   `yaml:"guacamole_base_url"`
	Cores             []string `yaml:"cores"`
	Port              int      `yaml:"port"`
	Redis             string   `yaml:"redis"`
	DB                DBConfig `yaml:"db"`
	GuacDB            DBConfig `yaml:"guac_db"`

	// 코어 선택 알고리즘 파라미터 (기본값은 startup.applyAllocDefaults에서 보정)
	CpuOvercommit  float64 `yaml:"cpu_overcommit"`   // vCPU 오버커밋 배수, ≤0이면 1.0
	MemReservePct  float64 `yaml:"mem_reserve_pct"`  // 0..1, 메모리 여유분 비율
	DiskReservePct float64 `yaml:"disk_reserve_pct"` // 0..1, 디스크 여유분 비율
}

type DBConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
}

// memory: GiB
// disk: GiB
// CPU: Logical Core num

type Core struct {
	IP          string           // 코어의 IP 주소
	Port        uint16           // 코어의 포트 번호
	CoreInfoIdx CoreInfo         //코어의 물리적인 정보
	IsAlive     bool             //Core가 살았는지 죽었는지 확인
	VMInfoIdx   map[UUID]*VMInfo // core 안에 VM 정보
	FreeMemory  uint32           //할당되지 않은 코어의 Memory 자원 MiB
	FreeCPU     uint32           //할당되지 않은 코어의 CPU 자원 논리 코어 수
	FreeDisk    uint32           //할당되지 않은 코어의 Disk 자원 MiB
}

type CoreInfo struct {
	Memory uint32 // 코어의 전체 메모리 MiB
	Cpu    uint32 // 코어의 전체 CPU 논리 코어 수
	Disk   uint32 // 코어의 전체 디스크 MiB
}

type VMInfo struct {
	IP_VM        string
	UUID         UUID
	GuacPassword string
	MacAddr      string
	Memory       uint32 // VM의 메모리 MiB
	Cpu          uint32 // VM의 CPU 논리 코어 수
	Disk         uint32 // VM의 디스크 MiB
}
