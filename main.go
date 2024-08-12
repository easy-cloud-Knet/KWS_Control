package main

import vms "github.com/easy-cloud-Knet/KWS_Control/VMS"



func main(){

	vms.HeartBeatSensor()


	
}

func init(){
	vms.InitializeDevices()

}