package vms

type VM struct {
    Name        string `json:"name"`
    Memory      int16   `json:"memory"`
    Disk        int16   `json:"disk"`
    IP          string  `json:"ip"`
    IsAlive     bool    `json:"isAlive"`
    IsAllocated bool    `json:"isAllocated"`
}

type Computer struct {
    Name      string `json:"name"`
    Allocated []VM  `json:"allocated"`
    IP        string    `json:"ip"`
    MAC       string    `json:"mac"`
    IsAlive   bool  `json:"isAlive"`
}

type machines interface{
    CheckRunning()
    GetStatus()
}


func InitializeDevices() {

	// config 파일이나 데이터베이스에서 읽어와야 함.
	//추후에 테라폼 같은 프로젝트 사용할 수도? 
    COM1 := Computer{
        Name:        "worker1",
        IP:          "192.168.64.5",
        IsAlive:     false,
    }
	COM2 := Computer{
        Name:        "worker2",
        IP:          "192.168.65.6",
        IsAlive:     false,
    }

    //     Name:      "sample Computer",
    //     Allocated: []VM{VM1,VM2},
    //     IP:        "127.0.0.1",
    //     IsAlive:   false,
    // }

    Computers = append(Computers, COM1, COM2)

}
