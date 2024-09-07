package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	WorkerCont "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)


func Server(portNum int, taskPool *WorkerCont.TaskHandler, contextStruct vms.InfraContext ){
	// main server와 통신하기 위한 http 서버
	// gin.DefaultWriter = io.Discard

	http.HandleFunc("/getStatus",func(w http.ResponseWriter, r *http.Request){
		// body, err := io.ReadAll(b.Body)
		// if err != nil{
		// 	log.Println(err)
		// }

		// defer b.Body.Close()
		//body.vmStatus
		//body.uuid
		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		workerControl:= &WorkerCont.TaskControl{
			ResultChann: make(chan WorkerCont.TaskExecutionResult),
		}
		newTask:=WorkerCont.Task{
			FunctionName: WorkerCont.GetStatus,	
			TaskSpecific: workerControl,
		}
		resultChannel:= newTask.TaskSpecific.ChanGetter()
		defer close(resultChannel)
		
		taskPool.WorkerAllocate(newTask)
		result:= <-resultChannel
		encoder := json.NewEncoder(w)
		encoder.Encode(result)
	})

	http.HandleFunc("GET /CreateVM",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerCont.Task{ 
			FunctionName: WorkerCont.CreateV,
	})
	})
	http.HandleFunc("GET /DeleteVM",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerCont.Task{ 
			FunctionName: WorkerCont.DeleteV,

		})
	})
	http.HandleFunc("GET /ConnectVM",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerCont.Task{ 
			FunctionName: WorkerCont.ConnectV,

		})
	})
	http.HandleFunc("GET /CheckVMHealth", func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerCont.Task{ 
			FunctionName: WorkerCont.UpdateStat,
		})
	})


	http.ListenAndServe(":"+strconv.Itoa(portNum), nil)



}