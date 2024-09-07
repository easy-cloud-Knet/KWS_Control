package vms

import (
	libvirt "libvirt.org/go/libvirt"
)




type  UUID string


type InfraContext struct{
     Computers []Computer
     VMPoolUnallocated []*VM
     VMPoolAllocated []*VM
     VMPool map[UUID]*VM
}

type InfraManage interface{
    UpdateList()
}

type VMInfo struct{
    State libvirt.DomainState `json:"state"`
    MaxMem uint64 `json:"maxmem"`
    Memory uint64 `json:"memory"`
    NrVirtCpu uint `json:"nrVirtCpu"`
    CpuTime uint64 `json:"cpuTime"`
    UUID UUID `json:"uuid"`
    IP string `json:"ip"`
}


type VM struct {
    VMInfo VMInfo `json:"vmInfo"`
    IsAlive     bool    `json:"isAlive"`
    IsAllocated bool    `json:"isAllocated"`
    IsLocatedAt Computer `json:"isLocatedAt"`
}

type Computer struct {
    Name      string `json:"name"`
    Allocated []VM  `json:"allocated"`
    IP        string    `json:"ip"`
    MAC       string    `json:"mac"`
    IsAlive   bool  `json:"isAlive"`
}

// type machines interface{
//     CheckRunning()
//     GetStatus()
// }


// type apiServer struct{
//     OutboundPort int `yaml: outboutPort`
//     TotalWorker int `yaml:totalWorker`
//     WorkLoadNum int `yaml:workLoadNum`
// }

// type workerInfo struct{
//     Name string `yaml:name`
//     Mac string  `yaml: mac`
//     port int `yaml:port`
// }

// type WorkerInfos []workerInfo
// yaml파일을 읽어와서 부팅함.

