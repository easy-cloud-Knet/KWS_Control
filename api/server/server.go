package api

import (
	"net/http"
	"strconv"

	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/taskPool"
)


func Server(portNum int, taskPool *WorkerConn.TaskHandler ){
	// main server와 통신하기 위한 http 서버
	// gin.DefaultWriter = io.Discard

	http.HandleFunc("GET /getStatus",func(w http.ResponseWriter, b *http.Request){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.GetStatus,
			Arguments : []interface{}{},		
	})
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