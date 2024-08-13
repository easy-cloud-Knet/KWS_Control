package main

import vms "github.com/easy-cloud-Knet/KWS_Control/vm"



func main(){

	vms.HeartBeatSensor()


	
}

func init(){
	vms.InitializeDevices()

}