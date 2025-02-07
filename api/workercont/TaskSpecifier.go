package WorkerCont

import (
	"encoding/json"
	"net/http"
	"sync"

	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)

type GetStatusParam struct {
	UUID string `json:"UUID"`
}

type DeletevmParam struct {
	UUID       string `json:"UUID"`
	DeleteType int    `json:"DeleteType"`
}

type CreateVMParam struct {
	DomType     string     `json:"domType"`
	DomName     string     `json:"domName"`
	Users       []UserInfo `json:"users"`
	UUID        string     `json:"uuid"`
	OS          string     `json:"os"`
	NetworkType string     `json:"netType"`
	HWInfo      HWInfo     `json:"HWInfo"`
	Network     Network
	Method      uint `json:"method"`
}

type UserInfo struct {
	UserName     string `json:"name"`
	UserGroup    string `json:"groups"`
	UserPassword string `json:"passWord"`
}

type HWInfo struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type Network struct {
	Ips     []string
	NetType int
}

//parameters for each functions

type Task struct {
	FunctionName functionName
	TaskSpecific TaskJustifier
}

type TaskWorker struct {
	taskLenMu   sync.Mutex
	tasksLength int
	workLoads   chan *Task
	workerNum   int
}

type TaskHandler struct {
	TaskHandlersList []*TaskWorker // 현재 돌아가고 있는 스레드를 가리키는 포인터
	workingIndex     int           // 현재 돌아가고 있는 스레드의 갯수
} //테스크 임베딩, 필드 추가 필요

type TaskControlCreateVM struct {
	ResultChan chan string
	UUID       vms.UUID
	Param      *CreateVMParam
	Vms        *vms.ControlInfra
	//추가 필요
}

type TaskControlDeleteVM struct {
	ResultChan chan string
	UUID       vms.UUID
	Param      *DeletevmParam
}

type TaskControlGetStatus struct {
	ResultChan chan TaskExecutionResult
	UUID       vms.UUID
}

type TaskExecutionResult struct {
	IsSuccess   bool
	InVMContext vms.VMInfo
}

type TaskJustifier interface {
	TaskUnparsor(r *http.Request) error
}

// functions for each structures, needed for interface
func (t *TaskControlCreateVM) TaskUnparsor(r *http.Request) error {
	var param CreateVMParam
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		return err
	}
	t.Param = &param
	return nil
}

func (t *TaskControlGetStatus) TaskUnparsor(r *http.Request) error {
	var param GetStatusParam
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		//에러 정의 필요
		return err
	}
	return nil
}

func (t *TaskControlDeleteVM) TaskUnparsor(r *http.Request) error {
	var param DeletevmParam
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		//에러 정의 필요
		return err
	}
	return nil
}
