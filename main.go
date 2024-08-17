package main

import (
	_ "os"

	api "github.com/easy-cloud-Knet/KWS_Control/api/server"
	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/taskPool"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)



func main(){
	aa:= make(chan int)
	var TaskHandlersPool WorkerConn.TaskHandler
	WorkerConn.InitWorkers(&TaskHandlersPool) 
	go vms.HeartBeatSensor()

	go api.Server(8080,&TaskHandlersPool)
	WorkerConn.PsudoRequestSender(&TaskHandlersPool)
	<-aa
}

func init(){
	vms.InitializeDevices()
	
}