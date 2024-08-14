package api

import (
	"strconv"

	WorkerConn "github.com/easy-cloud-Knet/KWS_Control/api/WorkerCont"
	"github.com/gin-gonic/gin"
)

func Server(portNum int, taskPool *WorkerConn.TaskHandler ){
	// main server와 통신하기 위한 http 서버
	router:=gin.Default()

	router.GET("/gin", func(c *gin.Context){
		taskPool.WorkerAllocate(WorkerConn.Task{ 
			function: WorkerConn.InitWorker,
			variable : []interface{}{},

		})
	})
	router.Run(":"+strconv.Itoa(portNum))
}