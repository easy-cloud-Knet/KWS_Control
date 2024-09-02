package vms

import (
	"github.com/easy-cloud-Knet/KWS_Control/api/requester"
)

func (c * Computer)UpdateVMList (VMPoolUnallocated []*VM, VMPoolallocated []*VM){
		var VMList []requester.VMList
	
		VMList=requester.GetVMList(c.IP)

		for _,i := range VMList{
			if i.IsAlive==true{

			}else{
				
			}
		}
	}
