package WorkerCont

import (
	"context"
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
	stat:=Task{ 
		FunctionName: GetStatus,
		Arguments : []interface{}{},
	}
	
	taskList:=[]Task{con,del,cre,up,stat} 
	fmt.Print(len(taskList))
	for j:=0; j<20; j++{
		workerHandler.WorkerAllocate(taskList[j%(len(taskList))])
	}
}




func (t * TaskWorker) UpdateStatusTest(context.Context){
	t.workDescription(UpdateStat)
	time.Sleep(5*time.Second)
}
func (t * TaskWorker) CreateVMTest(context.Context){
	t.workDescription(CreateV)
	time.Sleep(5*time.Second)
}
func (t * TaskWorker) ConnectVMTest(context.Context){
	t.workDescription(ConnectV)
	time.Sleep(5*time.Second)
}
func (t * TaskWorker) DeleteVMTest(context.Context){
	t.workDescription(DeleteV)
	time.Sleep(5*time.Second)
}
func (t * TaskWorker) GetStatusTest(context.Context){
	t.workDescription(DeleteV)
	time.Sleep(5*time.Second)
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
		case GetStatus:
			return "status of VM";
		default:
			return "undefined Task"
	}
}