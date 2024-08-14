package main

import (
	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/WorkerCont"
	api "github.com/easy-cloud-Knet/KWS_Control/api/server"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)



func main(){
	var TaskHandlersPool WorkerConn.TaskHandler
	WorkerConn.InitWorkers(&TaskHandlersPool) 
	vms.HeartBeatSensor()

	go api.Server(8080,&TaskHandlersPool )

	
}

func init(){
	vms.InitializeDevices()
	
}