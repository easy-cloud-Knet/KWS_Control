package WorkerCont

import (
	//"context"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)

func (t *TaskWorker) UpdateStatus() {

}

func (t *TaskWorker) CreateVM(CreateVM *TaskControlCreateVM, controlInfra *vms.ControlInfra) {
	fmt.Printf("VM Name: %s\n", CreateVM.Param.DomName)
	CreateVM.Param.Network.NetType = 0

	CreateVM.Param.Network.Ips = []string{"14.5.51.8", "12.5.28.8"}

	tempVMInfo := &vms.VMInfo{
		IP_VM:  []string{"14.5.51.8", "12.5.28.8"},
		UUID:   CreateVM.Param.UUID,
		Memory: CreateVM.Param.HWInfo.Memory,
		Cpu:    CreateVM.Param.HWInfo.CPU,
	}
	fmt.Println(tempVMInfo.UUID)
	fmt.Println(controlInfra)

	//controlInfra에 추가된 VM 정보 기입
	controlInfra.Cores[0].VMInfoIdx["cc13cdc79"] = tempVMInfo
	// JSON 변환
	jsonData, err := json.Marshal(CreateVM.Param)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	var apiURL = "http://223.194.20.119:28779/createVM"
	// HTTP POST 요청 생성
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		CreateVM.ResultChan <- fmt.Sprintf("Error sending request to API: %v", err)
		defer resp.Body.Close() // 응답 본문 닫기
		return
	}
	defer resp.Body.Close() // 응답 본문 닫기

	// 응답 확인
	if resp.StatusCode == http.StatusOK {
		CreateVM.ResultChan <- "VM successfully created and assigned!"
	} else {
		CreateVM.ResultChan <- fmt.Sprintf("Failed to create VM. Status code: %d", resp.StatusCode)
	}

	// 어느코어에 create 할 지 여기서 결정해야함.
	// 알고리즘에 의해 어느 코어에 할당할 지 결정되었다고 가정

}
func (t *TaskWorker) ConnectVM() {
}
func (t *TaskWorker) DeleteVM(DeleteVM *TaskControlDeleteVM, controlInfra *vms.ControlInfra) {
	jsonData, err := json.Marshal(DeleteVM.Param)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Printf("UUID: %s\n", DeleteVM.Param.UUID)
	var apiURL = "http://223.194.20.119:28779/DeleteVM"
	// HTTP POST 요청 생성
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		DeleteVM.ResultChan <- fmt.Sprintf("Error sending request to API: %v", err)
		return
	}
	defer resp.Body.Close() // 응답 본문 닫기

	// 응답 확인
	if resp.StatusCode == http.StatusOK {
		DeleteVM.ResultChan <- "VM successfully Deleted and assigned!"

	} else {
		DeleteVM.ResultChan <- fmt.Sprintf("Failed to create VM. Status code: %d", resp.StatusCode)
	}
}

func (t *TaskWorker) GetStatus(task *Task) {

}
