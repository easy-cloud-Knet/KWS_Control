package WorkerCont

import (
	"fmt"
	"time"
)

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
	FunctionName functionName 
	Arguments []interface{}
}

type TaskWorker struct{
	tasksLength int8
	workLoads  chan Task
	workerNum int
}

type TaskHandler struct{
	TaskHandlersList []*TaskWorker
	TaskPool []*Task
	workingIndex int
}

func InitWorkers(pool *TaskHandler){
	//pool *TaskHandler as args
	pool.TaskHandlersList =make([]*TaskWorker, NUM_OF_TASK_HANDLER)
	pool.workingIndex=0
	pool.TaskPool=make([]*Task,10)
	
	for i :=0;i< NUM_OF_TASK_HANDLER; i++{
		pool.TaskHandlersList[i]= &TaskWorker{
			tasksLength:0,
			workLoads: make(chan Task,NUM_OF_ALL_WORKLOAD),
			workerNum: i,
		}
		go pool.TaskHandlersList[i].StartWorking()

	}
}

func (t*TaskWorker) StartWorking(){
	for{
		select  {
			case work,ok:=<-t.workLoads:{
				if !ok{
					fmt.Println("channel closed")
					return;
				}else{
					switch work.FunctionName{
					case CreateV:
						t.tasksLength--
						t.CreateVMTest()
					case UpdateStat:
						t.tasksLength--	
						t.UpdateStatusTest()
					case ConnectV:
						t.tasksLength--
						t.ConnectVMTest()
					case DeleteV:
						t.tasksLength--
						t.DeleteVMTest()
					default:
						fmt.Printf("undefined task")
					}
				}
			}
		default:
			fmt.Println("work Done, waiting")
			time.Sleep(time.Second*3)
		}
	}
	}
	



func (t *TaskHandler)WorkerAllocate(task Task, ){
	t.workingIndex=t.workingIndex%NUM_OF_TASK_HANDLER
	t.TaskHandlersList[t.workingIndex].workLoads<-task
	t.TaskHandlersList[t.workingIndex].tasksLength++;
	t.workingIndex++
	
}




