package WorkerCont

import vms "github.com/easy-cloud-Knet/KWS_Control/VMS"

//각각의 worker노드와 통신하기 위한 클라이언트 서버
type functionName int32

const NUM_OF_TASK_HANDLER=5
const NUM_OF_ALL_WORKLOAD=10


const(
	InitWorker functionName = iota +1
	UpdateStat 
	CreateV
	ConnectV
	DeleteV
)


type Task struct{
	function functionName 
	arguments []interface{}
}

type TaskWorker struct{
	tasksLength int8
	workLoads  chan Task
}

type TaskHandler struct{
	TaskHandlersList []*TaskWorker
	TaskPool []*Task
	workingIndex int
}

func InitWorkers(pool *TaskHandler){
	//pool *TaskHandler as args
	pool.TaskHandlersList =make([]*TaskWorker, NUM_OF_TASK_HANDLER)
	pool.TaskPool=make([]*Task,0)
	for i := range pool.TaskHandlersList{
		pool.TaskHandlersList[i]= &TaskWorker{
			tasksLength:0,
			workLoads: make(chan Task,NUM_OF_ALL_WORKLOAD),
		}
		go pool.TaskHandlersList[i].StartWorking()

	}
}
func (t*TaskWorker) StartWorking(){
	for{
		work:=<-t.workLoads
		select {
		case 
		:
			
		}
	
	}
	}

func (t * TaskWorker) updateStatus(){
	computerList := vms.Computers

}
func (t * TaskWorker) createVM(){
	computerList := vms.Computers

}
func (t * TaskWorker) connectVM(){
	computerList := vms.Computers

}
func (t * TaskWorker) deleteVM(){
	computerList := vms.Computers

}


func (t *TaskHandler)WorkerAllocate(task Task, ){
	//task Task 를 아규먼트로 받음
	//좀 더 효율적으로 만들필요 있음
	if t.workingIndex!=NUM_OF_TASK_HANDLER{
		t.workingIndex++
		t.TaskHandlersList[t.workingIndex].workLoads<-task
		t.TaskHandlersList[t.workingIndex].tasksLength++;
	}else{
		t.workingIndex=0
		t.TaskHandlersList[t.workingIndex].workLoads<-task
		t.TaskHandlersList[t.workingIndex].tasksLength++;
	}
}


// func functionEmmitor(task Task){
// 	switch(task.function){
// 		case CreateV:
// 			return ;
// 		case UpdateStat:
// 			return ;
// 		case ConnectV:
// 			return ;
// 		case DeleteV:
// 			return ;
// 		default:
// 	}
// }

