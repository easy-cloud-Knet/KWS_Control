package WorkerCont

import (
	"context"
	"fmt"
	"sync"
	"time"

	WorkerCont "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
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
	GetStatus
)


type Task[T any] struct{
	FunctionName functionName 
	TaskSpecific TaskJustifier[T]
}

type TaskControl[T any] struct{
	Arguments []interface{}
	ResultChann chan T
}

type TaskInfraControlResult struct{
	IsSuccess bool
	InfraContext vms.InfraContext 
}

type TaskVMControlResult struct{
	IsSuccess bool
	InVMContext vms.VMInfo 
}


type TaskJustifier[T any] interface{
	ChanGetter() chan T
	ChanSetter()
}

type TaskWorker struct{
	taskLenMu sync.Mutex
	tasksLength int
	workLoads  chan Task[any]
	workerNum int
}

type TaskHandler struct{
	TaskHandlersList []*TaskWorker
	workingIndex int
}

func (t *TaskControl[T]) ChanGetter() chan T{
	if t.ResultChann==nil{
		t.ChanSetter()
	}
	return t.ResultChann	
}
func (t *TaskControl[T]) ChanSetter(){
	vmChan :=make(chan T)
	t.ResultChann=vmChan
}




func InitWorkers(pool *TaskHandler){
	//pool *TaskHandler as args
	pool.TaskHandlersList =make([]*TaskWorker, NUM_OF_TASK_HANDLER)
	pool.workingIndex=0
	// pool.TaskPool=make([]*Task,10)
	
	for i :=0;i< NUM_OF_TASK_HANDLER; i++{
		pool.TaskHandlersList[i]= &TaskWorker{
			tasksLength:0,
			workLoads: make(chan Task[any],NUM_OF_ALL_WORKLOAD),
			workerNum: i,
		}
		go pool.TaskHandlersList[i].StartWorking()

	}
}

func (t*TaskWorker) StartWorking(){
	for{
		ctx:= context.Background()
		select  {
			case work,ok:=<-t.workLoads:{
				if !ok{
					fmt.Println("channel closed")
					return;
				}else{
					switch work.FunctionName{
					case CreateV:
						t.CreateVMTest(ctx)
						t.tasksLength--
					case UpdateStat:
						t.UpdateStatusTest(ctx)
						t.tasksLength--	
					case ConnectV:
						t.ConnectVMTest(ctx)
						t.tasksLength--
					case DeleteV:
						t.DeleteVMTest(ctx)
						t.tasksLength--
					case GetStatus:
						t.GetStatus(ctx)
						t.tasksLength--
					default:
						fmt.Printf("undefined task")
					}
				}
			}
		default:
			// fmt.Println("work Done, waiting")
			time.Sleep(time.Microsecond*300)
		}
	}
	}
	

func (t *TaskHandler) WorkerAllocate(task WorkerCont.TaskInfraControlResult) {
	for {
		workerIndex := t.workingIndex % NUM_OF_TASK_HANDLER
		worker := t.TaskHandlersList[workerIndex]
			
		worker.taskLenMu.Lock()
		if worker.tasksLength < 9 {
			worker.workLoads <- task
			worker.tasksLength++
			fmt.Printf("Allocated task to worker %d. Current tasks: %d\n", workerIndex, worker.tasksLength)
			worker.taskLenMu.Unlock()
			t.workingIndex = (t.workingIndex + 1) % NUM_OF_TASK_HANDLER
			return
		}
		worker.taskLenMu.Unlock()
		t.workingIndex = (t.workingIndex + 1) % NUM_OF_TASK_HANDLER
	}
}




