package requester

import (
	"net/http"
)

type VMList struct {
	VMName string
	IsAlive bool
}

func GetVMList(IP string) []VMList {
	resp , err := http.Get(IP+"/getStatus")
	if err!= nil{
		panic(err)
	}
	//resp handle


	
}
