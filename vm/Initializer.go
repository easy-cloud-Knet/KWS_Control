package vms

import (
	_ "gopkg.in/yaml.v3"
)





func InitializeDevices() {

	// config 파일이나 데이터베이스에서 읽어와야 함.
	//추후에 테라폼 같은 프로젝트 사용할 수도? 
    var Computers []Computer
    var VMPoolUnallocated []*VM
    var VMPoolAllocated []*VM


    COM1 := Computer{
        Name:        "worker1",
        IP:          "192.168.64.5",
        IsAlive:     false,
    }
	COM2 := Computer{
        Name:        "worker2",
        IP:          "192.168.64.6",
        IsAlive:     false,
    }



    Computers = append(Computers, COM1, COM2)

    for _,c :=range Computers{
        c.UpdateVMList(VMPoolAllocated,VMPoolUnallocated)
    }

	go HeartBeatSensor(Computers)

}
