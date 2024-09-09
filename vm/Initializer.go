package vms

import (
	"fmt"

	_ "gopkg.in/yaml.v3"
)





func InitializeDevices(InfraCon *InfraContext) {

	// config 파일이나 데이터베이스에서 읽어와야 함.
	//추후에 테라폼 같은 프로젝트 사용할 수도? 

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



    InfraCon.Computers = append(InfraCon.Computers,COM1,  COM2)
    fmt.Println("hellot1")
    InfraCon.UpdateList()
    fmt.Println("hellot2")

	go HeartBeatSensor(InfraCon.Computers)

}
