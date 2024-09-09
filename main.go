package main

import (
	"fmt"
	_ "os"

	api "github.com/easy-cloud-Knet/KWS_Control/api/server"
	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)



func main(){

	contextStruct:= vms.InfraContext{
		Computers: []vms.Computer{},
		VMPoolUnallocated: []*vms.VM{},
		VMPoolAllocated:[]*vms.VM{},
		VMPool:map[vms.UUID]*vms.VM{},
		}
	
	fmt.Println("hellot")
	

	var TaskHandlersPool WorkerConn.TaskHandler
	WorkerConn.InitWorkers(&TaskHandlersPool,) 
	vms.InitializeDevices(&contextStruct)
	go api.Server(8080,&TaskHandlersPool, &contextStruct)
	WorkerConn.PsudoRequestSender(&TaskHandlersPool)

	select {}
}
