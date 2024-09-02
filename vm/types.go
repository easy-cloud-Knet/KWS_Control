package vms


type VM struct {
    Name        string `json:"name"`
    Memory      int16   `json:"memory"`
    Disk        int16   `json:"disk"`
    IP          string  `json:"ip"`
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

type machines interface{
    CheckRunning()
    GetStatus()
}


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

