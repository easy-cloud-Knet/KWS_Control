package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)


func Server(portNum int, taskPool *WorkerConn.TaskHandler, contextStruct vms.InfraContext ){
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
		workerControl:= &WorkerConn.TaskControl[WorkerConn.TaskInfraControlResult]{
			ResultChann: make(chan WorkerConn.TaskInfraControlResult),
		}
		newTask:=WorkerConn.Task[WorkerConn.TaskInfraControlResult]{
			FunctionName: WorkerConn.GetStatus,	
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
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.CreateV,
			Arguments : []interface{}{},		
	})
	})
	http.HandleFunc("GET /DeleteVM",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.DeleteV,
			Arguments : []interface{}{},

		})
	})
	http.HandleFunc("GET /ConnectVM",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.ConnectV,
			Arguments : []interface{}{},

		})
	})
	http.HandleFunc("GET /CheckVMHealth", func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.UpdateStat,
			Arguments : []interface{}{},

		})
	})


	http.ListenAndServe(":"+strconv.Itoa(portNum), nil)



}