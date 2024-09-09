package vms

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)


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
	info:=[]VMInfo{}
	err=json.Unmarshal(body,&info)
	if err!=nil{
		log.Panic(err)
	}
	fmt.Println(info)
}



func (i * InfraContext)UpdateList(){
	 go func(){
		 for _ , c := range i.Computers{
			c.GetVMList(i.VMPool)
	}}()
}





// func GetVMListAll() []VMList{
	
// }