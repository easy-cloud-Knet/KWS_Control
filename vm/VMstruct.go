package vms

import (
	_ "libvirt.org/go/libvirt"
)

type UUID string

type ControlInfra struct {
	Cores      []Core `json:"Cores"`
	VMLocation map[UUID]*Core
}

type Core struct {
	IP          string           `json:"IP"`
	CoreInfoIdx CoreInfo         `json:"CoreInfoIdx"`
	IsAlive     bool             `json:"IsAlive"`
	VMInfoIdx   map[UUID]*VMInfo `json:"VMInfoIdx"`
	FreeMemory  int              `json:"FreeMemory"`
	FreeCPU     int              `json:"FreeCPU"`
}

type CoreInfo struct {
	Memory int `json:"Memory"`
	Cpu    int `json:"Cpu"`
	Disk   int `json:"Disk"`
}

type VMInfo struct {
	IP_VM  []string `json:"IP_VM"`  //
	UUID   UUID     `json:"UUID"`   //
	Memory int      `json:"Memory"` //
	Cpu    int      `json:"Cpu"`    //
	Disk   int      `json:"Disk"`   //
}

// ----------------------------------------------------
// // 모든 VM의 상태를 관리
// type InfraContext struct {
// 	Computers       []Computer   //관리중인 컴퓨터의 목록
// 	VMPoolAllocated []*VM        //할당된 VM의 목록
// 	VMPool          map[UUID]*VM //UUID를 활용한 VM의 전체 맵
// }

// //모든 컴퓨터, vm에 대한 정보를 담고 있음.
// // json이나 yaml에 지속적으로 업데이트해서 재부팅시에도 유지하는 것 필수

// type InfraManage interface {
// 	UpdateList()
// }

// type VM struct {
// 	VMInfo      VMInfo
// 	IsAlive     bool
// 	IsAllocated bool
// 	IsLocatedAt Computer
// }

// // VM 내부의 상세 정보 표시
// // 사용자가 원하는 하드웨어 정보를 받으면 이 구조체에 저장 후 사용
// type VMInfo struct {
// 	MaxMem    uint64
// 	Memory    uint64
// 	NrVirtCpu uint
// 	CpuTime   uint64
// 	UUID      UUID
// 	IP        string
// }

// // Core의 정보들이 저장되어있음
// type Computer struct {
// 	Name      string
// 	Allocated []VM
// 	IP        string
// 	MAC       string
// 	IsAlive   bool
// }

//
