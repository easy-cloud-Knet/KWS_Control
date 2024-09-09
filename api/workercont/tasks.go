package WorkerCont

import (
	"context"
	"fmt"
)


func (t * TaskWorker) UpdateStatus(){
	
}
func (t * TaskWorker) CreateVM(){
}
func (t * TaskWorker) ConnectVM(){
}
func (t * TaskWorker) DeleteVM(){
}
func (t * TaskWorker) GetStatus(ctx context.Context,task *Task){
	fmt.Println(task.FunctionName)

	if task.TaskSpecific==nil {
		return;
	}else{
	arguments:=task.TaskSpecific.ArgumentsGetter()
	if len(arguments)==0{
		fmt.Printf("sibal")
	}}
	//computers list :0 
	//vmstatus :1
	//uuid :2
// 	value,ok:=arguments[0].(vms.InfraContext)
// 	if ok{
// 	value.UpdateList()

// 	}else{
// log.Panic("sibal")}
}
