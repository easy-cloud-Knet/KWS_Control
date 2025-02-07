package server

import (
	"encoding/json"
	"fmt"

	//"fmt"
	//"io"
	"net/http"
	"strconv"

	WorkerCont "github.com/easy-cloud-Knet/KWS_Control/api/workercont"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)

func Server(portNum int, taskPool *WorkerCont.TaskHandler, contextStruct *vms.ControlInfra) error {
	// main server와 통신하기 위한 http 서버
	// gin.DefaultWriter = io.Discard
	http.HandleFunc("Get /getStatus", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		workerControl := &WorkerCont.TaskControlGetStatus{
			ResultChan: make(chan WorkerCont.TaskExecutionResult),
		}
		resultChannel := workerControl.ResultChan
		defer close(resultChannel)
		workerControl.TaskUnparsor(r)

		newTask := &WorkerCont.Task{
			FunctionName: WorkerCont.GetStatus,
			TaskSpecific: workerControl,
		}

		taskPool.WorkerAllocate(newTask)
		result := <-resultChannel
		encoder := json.NewEncoder(w)
		encoder.Encode(result)
	})

	http.HandleFunc("/CreateVM", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { // POST로 요청 제한
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		param, err := util.UnmarshalBodyAndClose[WorkerCont.CreateVMParam](r.Body)
		if err != nil {
			http.Error(w, "Failed to read or parse JSON", http.StatusBadRequest)
			return
		}
		param.Network.Ips = []string{"14.5.51.8", "12.5.28.8"}
		param.Network.NetType = 0
		task := WorkerCont.NewCreateVMTask(&vms.Core{IP: "223.194.20.119", Port: 28779}, param) // TODO: core assignment
		resp, err := task.Await()
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Failed to create VM", http.StatusInternalServerError)
			return
		}

		encoder := json.NewEncoder(w)
		if err = encoder.Encode(resp); err != nil {
			http.Error(w, "Failed to encode result", http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/DeleteVM", func(w http.ResponseWriter, r *http.Request) {
		param, err := util.UnmarshalBodyAndClose[WorkerCont.CreateVMParam](r.Body)
		if err != nil {
			http.Error(w, "Failed to read or parse JSON", http.StatusBadRequest)
			return
		}
		task := WorkerCont.NewDeleteVMTask(&vms.Core{IP: "10.5.12.2", Port: 8080}, param)
		resp, err := task.Await()
		if err != nil {
			http.Error(w, "Failed to create VM", http.StatusInternalServerError)
			return
		}
		encoder := json.NewEncoder(w)
		if err = encoder.Encode(resp); err != nil {
			http.Error(w, "Failed to encode result", http.StatusInternalServerError)
		}
	})
	http.HandleFunc("GET /ConnectVM", func(w http.ResponseWriter, b *http.Request) {
		taskPool.WorkerAllocate(&WorkerCont.Task{
			FunctionName: WorkerCont.ConnectV,
		})
	})
	http.HandleFunc("GET /CheckVMHealth", func(w http.ResponseWriter, b *http.Request) {
		taskPool.WorkerAllocate(&WorkerCont.Task{
			FunctionName: WorkerCont.UpdateStat,
		})
	})

	err := http.ListenAndServe(":"+strconv.Itoa(portNum), nil)
	if err != nil {
		return err
	}

	return nil
}

/*
회의 해봐야 하는 내용들
1. VM 생성 완료 했을 때 벡에다가 리턴해야 하는게 뭔지?
2. Core 컴퓨터가 실행되면 Control에 Core 정보 보내줘야함.

*/
