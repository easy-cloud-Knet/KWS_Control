package main

import (
	"fmt"
	_ "os"

	//"sync"
	api "github.com/easy-cloud-Knet/KWS_Control/api/server"
	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)

func main() {
	var TaskHandlersPool WorkerConn.TaskHandler
	WorkerConn.InitWorkers(&TaskHandlersPool)                         //스레드를 초기화하는 함수
	contextStruct, err := vms.InitializeDevices("./vm/database.json") //VM을 정의하는 함수
	if err != nil {
		fmt.Println("Error loading infrastructure:", err)
		return
	}
	// contextStruct.Cores[0].VMInfoIdx["213"] = &vms.VMInfo{
	// 	IP_VM:  []string{"192.168.1.101"},
	// 	UUID:   "vm-uuid-1234",
	// 	Memory: 4096,
	// 	Cpu:    2,
	// 	Disk:   50,
	// }
	// // fmt.Printf("%s\n", contextStruct.VMLocation["vm-uuid-5678"].IP)
	// 초기화된 데이터 확인
	fmt.Printf("Loaded ControlInfra: %+v\n", contextStruct)
	//api.Lock()
	go func() {

		err := api.Server(8080, &TaskHandlersPool, contextStruct)
		//wg.Wait()
		//api.Unlock()
		if err != nil {
			panic(err)
		}
	}()

	//api.Done()
	//WorkerConn.PseudoRequestSender(&TaskHandlersPool)

	select {}
}
