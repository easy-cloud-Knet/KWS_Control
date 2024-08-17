package api

import (
	"io"
	"strconv"

	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/taskPool"
	"github.com/gin-gonic/gin"
)

func Server(portNum int, taskPool *WorkerConn.TaskHandler ){
	// main server와 통신하기 위한 http 서버
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	router:=gin.Default()



	router.GET("/CreateVM", func(c *gin.Context){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.CreateV,
			Arguments : []interface{}{},		
	})
	})

	router.GET("/DeleteVM", func(c *gin.Context){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.DeleteV,
			Arguments : []interface{}{},

		})
	})
	router.GET("/ConnectVM", func(c *gin.Context){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.ConnectV,
			Arguments : []interface{}{},

		})
	})
	router.GET("/CheckVMHealth", func(c *gin.Context){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			FunctionName: WorkerConn.UpdateStat,
			Arguments : []interface{}{},

		})
	})

	router.Run(":"+strconv.Itoa(portNum))
}