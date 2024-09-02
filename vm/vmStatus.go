package vms

import (
	"fmt"
	"net/http"
)


func (c * Computer)UpdateVMList (VMPoolUnallocated []*VM, VMPoolallocated []*VM){

		VMList:=GetVMList(c.IP)

		for _,i := range VMList{
			if i.IsAlive==true{

			}else{
				
			}
		}
	}



func GetVMList(IP string) []VM {
	resp , err := http.Get(IP+"/getStatus")
	if err!= nil{
		panic(err)
	}
	//resp handle
	fmt.Println(resp)

	var VML []VM
	return VML
}

