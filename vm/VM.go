package vms

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

func (i * InfraContext)UpdateList(onceSync *sync.Once){
	onceSync.Do( func(){
		 for _ , c := range i.Computers{
			c.GetVMList(i.VMPool)
	}
})
}




func (c *Computer)GetVMList(VMList map[UUID]*VM) {
	if !strings.HasPrefix(c.IP, "http://") && !strings.HasPrefix(c.IP, "https://") {
		c.IP = "http://" + c.IP
	}

	resp, err := http.Get(c.IP+":8080"+ "/getStatus")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close() 

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
}


// func GetVMListAll() []VMList{
	
// }