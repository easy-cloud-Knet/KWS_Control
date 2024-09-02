package main

import (
	_ "os"

	api "github.com/easy-cloud-Knet/KWS_Control/api/server"
	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/taskPool"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)



func main(){
	var TaskHandlersPool WorkerConn.TaskHandler
	WorkerConn.InitWorkers(&TaskHandlersPool) 

	go api.Server(8080,&TaskHandlersPool)
	WorkerConn.PsudoRequestSender(&TaskHandlersPool)

	select {}
}

func init(){
	vms.InitializeDevices()
	
}