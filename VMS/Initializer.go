package vms

type VM struct {
    Name        string
    Memory      int16
    Disk        int16
    IP          string
    IsAlive     bool
    IsAllocated bool
}

type Computer struct {
    Name      string
    Allocated []VM
    IP        string
    MAC       string
    IsAlive   bool
}

type machines interface{
    CheckRunning()
}


func InitializeDevices() {

	// config 파일이나 데이터베이스에서 읽어와야 함.
	//추후에 테라폼 같은 프로젝트 사용할 수도? 
    VM1 := VM{
        Name:        "sample VM",
        Memory:      6,
        Disk:        20,
        IP:          "192.168.64.5",
        IsAlive:     false,
        IsAllocated: false,
    }
	VM2 := VM{
        Name:        "sample VM",
        Memory:      6,
        Disk:        20,
        IP:          "192.168.65.5",
        IsAlive:     false,
        IsAllocated: false,
    }

    COM1 := Computer{
        Name:      "sample Computer",
        Allocated: []VM{VM1,VM2},
        IP:        "127.0.0.1",
        IsAlive:   false,
    }

    Computers = append(Computers, COM1)
}
