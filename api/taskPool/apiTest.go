package WorkerCont

import (
	"fmt"
	_ "log"
	"time"
)


func PsudoRequestSender(workerHandler *TaskHandler){
	con:=Task{ 
			FunctionName: ConnectV,
			Arguments : []interface{}{},
		}
	del:=Task{ 
		FunctionName: DeleteV,
		Arguments : []interface{}{},
	}
	cre:=Task{ 
		FunctionName: CreateV,
		Arguments : []interface{}{},
	}
	up:=Task{ 
		FunctionName: UpdateStat,
		Arguments : []interface{}{},
	}
	taskList:=[]Task{con,del,cre,up} 
	fmt.Print(len(taskList))
	for j:=0; j<40; j++{
		workerHandler.WorkerAllocate(taskList[j%(len(taskList))])
	}
}




func (t * TaskWorker) UpdateStatusTest(){
	t.workDescription(UpdateStat)
	time.Sleep(1*time.Second)
}
func (t * TaskWorker) CreateVMTest(){
	t.workDescription(CreateV)
	time.Sleep(2*time.Second)
}
func (t * TaskWorker) ConnectVMTest(){
	t.workDescription(ConnectV)
	time.Sleep(3*time.Second)
}
func (t * TaskWorker) DeleteVMTest(){
	t.workDescription(DeleteV)
	time.Sleep(4*time.Second)
}


func (w * TaskWorker) workDescription(workName functionName){
	fmt.Println("*******************************************************")
	fmt.Printf("worker %d \n", w.workerNum)
	fmt.Printf("current length of workLoad is %d currently working on %s \n",w.tasksLength, functionNameEmmitor(workName))
	fmt.Println("*******************************************************")
}

func functionNameEmmitor(functionNum functionName) string{
	switch(functionNum){
	case CreateV:
			return "CreateVM";
		case UpdateStat:
			return "Update status";
		case ConnectV:
			return "connect VM";
		case DeleteV:
			return "delete VM";
		default:
			return "undefined Task"
	}
}