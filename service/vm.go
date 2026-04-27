package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/pkg/guacamole"
	internalssh "github.com/easy-cloud-Knet/KWS_Control/pkg/ssh"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

// 새 VM 만드는 무언가.
// 자원 많이 남은 코어를 찾고, 리소스 할당 업데이트, ControlContext 상태 업데이트.
func CreateVM(req model.CreateVMRequest, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()

	log.Info("func CreateVM() memory=%d GiB, cpu=%d, disk=%d GiB", req.HardwareInfo.Memory, req.HardwareInfo.CPU, req.HardwareInfo.Disk, true)

	// err = validateCreateVMRequest(req)

	// guacamole 부분 필요

	// 적합한 코어 찾기
	log.DebugInfo("core selection process. req: memory=%d GiB, cpu=%d, disk=%d",
		req.HardwareInfo.Memory, req.HardwareInfo.CPU, req.HardwareInfo.Disk)

	var selectedCore *vms.Core = nil
	selectedCoreIndex := -1
	aliveCount := 0

	// Cores/FreeMemory 등 공유 상태를 읽으므로 RLock (동시 읽기는 허용, 쓰기만 차단)
	contextStruct.RLock()
	for i := range contextStruct.Cores {

		core := &contextStruct.Cores[i]
		log.DebugInfo("core %s checking: FreeMemory=%d, FreeCPU=%d, FreeDisk=%d, IsAlive=%t", core.IP, core.FreeMemory, core.FreeCPU, core.FreeDisk, core.IsAlive)

		if !core.IsAlive {
			log.DebugWarn("core %s is not alive, skipping", core.IP)
			continue
		}
		aliveCount++

		memoryOk := core.FreeMemory >= req.HardwareInfo.Memory
		cpuOk := core.FreeCPU >= req.HardwareInfo.CPU
		diskOk := core.FreeDisk >= req.HardwareInfo.Disk

		if req.HardwareInfo.Disk == 0 {
			log.DebugError("test")
		}

		if !memoryOk {
			log.DebugWarn("%s !memoryOk: req=%d, available=%d", core.IP, req.HardwareInfo.Memory, core.FreeMemory)
		}
		if !cpuOk {
			log.DebugWarn("%s !cpuOk   : req=%d, available=%d", core.IP, req.HardwareInfo.CPU, core.FreeCPU)
		}
		if !diskOk {
			log.DebugWarn("%s !diskOk  : req=%d, available=%d", core.IP, req.HardwareInfo.Disk, core.FreeDisk)
		}

		if memoryOk && cpuOk && diskOk {
			selectedCore = core
			selectedCoreIndex = i
			log.DebugInfo("core found: %s", selectedCore.IP)
			break
		} else {
			log.DebugInfo("%s rejected: memory=%t, cpu=%t, disk=%t", core.IP, memoryOk, cpuOk, diskOk)
		}
	}
	// 코어 탐색 완료 후 즉시 해제 — 이후 CMS/Guacamole 네트워크 콜은 락 없이 진행
	contextStruct.RUnlock()

	if selectedCore == nil {
		log.Error("No suitable core found! Total cores: %d, Alive cores: %d, Required: Memory=%d CPU=%d Disk=%d",
			len(contextStruct.Cores), aliveCount, req.HardwareInfo.Memory, req.HardwareInfo.CPU, req.HardwareInfo.Disk, true)

		if aliveCount > 0 {
			log.DebugError("alive cores:")
			for i := range contextStruct.Cores {
				core := &contextStruct.Cores[i]
				if core.IsAlive {
					log.DebugError("  %s: Memory=%d/%d, CPU=%d/%d, Disk=%d/%d",
						core.IP, core.FreeMemory, core.CoreInfoIdx.Memory, core.FreeCPU, core.CoreInfoIdx.Cpu, core.FreeDisk, core.CoreInfoIdx.Disk)
				}
			}
		} else {
			log.DebugError("no alive cores available")
		}

		return fmt.Errorf("CreateVM: no suitable core found")
	}
	var privateKeyPEM, publicKeyOpenSSH, err = internalssh.GenerateSSHKey()
	if err != nil {
		log.Error("GenerateSshKey() failed: %v", err, true)
		return fmt.Errorf("CreateVM: failed to generate SSH key: %w", err)
	}

	// add : back -> vm uuid    ->  cms   다른 api
	// new : subnet 찾기 -> cms

	// 문제가 생겼을 때 지우는 무언가
	var guacamoleConfigured = false
	var coreResourcesAllocated = false
	var newSubnetAllocated = false
	var uuid = vms.UUID(req.UUID.String().(string))

	cleanup := func() {
		if guacamoleConfigured {
			log.Info("clean up clean up")
			if cleanupErr := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); cleanupErr != nil {
				log.Error("Failed to cleanup Guacamole config during rollback: %v", cleanupErr)
			}
		}
		if coreResourcesAllocated {
			// VMInfoIdx와 Free* 필드는 여러 핸들러가 동시에 접근하므로 Lock 필요
			contextStruct.Lock()
			delete(selectedCore.VMInfoIdx, uuid)
			selectedCore.FreeMemory += req.HardwareInfo.Memory
			selectedCore.FreeCPU += req.HardwareInfo.CPU
			selectedCore.FreeDisk += req.HardwareInfo.Disk
			contextStruct.Unlock()
		}
		if newSubnetAllocated {
			//subnet--
		}
	}

	var subnetReq *client.NewSubnetRequest
	cmsClient := client.NewCmsClient()

	if req.Subnettype == "Add" {
		subnetReq, err = AddCmsSubnet(cmsClient, contextStruct, uuid)
	} else {
		subnetReq, err = NewCmsSubnet(cmsClient, contextStruct)
		newSubnetAllocated = true
	}
	if err != nil {
		log.Error("CreateVM: failed to configure cms: %v", err, true)
		return fmt.Errorf("CreateVM: failed to configure cms: %w", err)
	}

	fmt.Printf("%s\n", subnetReq.IP)
	fmt.Printf("%s\n", subnetReq.MacAddr)
	fmt.Printf("%s\n", subnetReq.SdnUUID)

	userPass := guacamole.Configure(req.Users[0].Name, string(req.UUID), subnetReq.IP, privateKeyPEM, contextStruct.GuacDB)

	if userPass == "" {
		log.Error("CreateVM: failed to configure Guacamole", true)
		return fmt.Errorf("CreateVM: failed to configure Guacamole")
	}
	guacamoleConfigured = true

	newVM := &vms.VMInfo{
		UUID:         uuid,
		GuacPassword: userPass,
		MacAddr:      subnetReq.MacAddr,
		Memory:       req.HardwareInfo.Memory,
		Cpu:          req.HardwareInfo.CPU,
		Disk:         req.HardwareInfo.Disk,
		IP_VM:        subnetReq.IP,
	}

	// VMInfoIdx map 쓰기 + Free* 필드 감소는 원자적으로 처리해야 함
	// (다른 CreateVM goroutine이 Free* 읽고 같은 코어 선택하는 race 방지)
	contextStruct.Lock()
	if selectedCore.VMInfoIdx == nil {
		selectedCore.VMInfoIdx = make(map[vms.UUID]*vms.VMInfo)
	}
	selectedCore.VMInfoIdx[uuid] = newVM
	selectedCore.FreeMemory -= req.HardwareInfo.Memory
	selectedCore.FreeCPU -= req.HardwareInfo.CPU
	selectedCore.FreeDisk -= req.HardwareInfo.Disk
	coreResourcesAllocated = true
	contextStruct.Unlock()
	// 이후 HTTP 전송은 락 밖에서 — 네트워크 콜 중 락을 잡으면 다른 요청 전체가 블로킹됨

	log.DebugInfo("core %s updated: FreeMemory=%d, FreeCPU=%d, FreeDisk=%d", selectedCore.IP, selectedCore.FreeMemory, selectedCore.FreeCPU, selectedCore.FreeDisk)

	req.NetConf.Ips = []string{subnetReq.IP}
	req.SdnUUID = subnetReq.SdnUUID
	req.MacAddr = subnetReq.MacAddr
	req.NetConf.NetType = 0
	req.Users[0].SSHAuthorizedKeys = []string{publicKeyOpenSSH}

	vmRedisInfo := model.VMRedisInfo{
		UUID:   uuid,
		CPU:    req.HardwareInfo.CPU,
		Memory: req.HardwareInfo.Memory,
		Disk:   req.HardwareInfo.Disk,
		IP:     subnetReq.IP,
		Status: model.VMStatusUnknown, // prepare begin 으로 초기화 함
		Time:   time.Now().Unix(),
	}

	// HTTP 전송 전에 저장을 완료하여 Core에서 업데이트할 수 있도록 순서 보장
	if err := StoreVMInfoToRedis(context.Background(), rdb, vmRedisInfo); err != nil {
		log.Warn("failed to store VM info to redis: %v", err, true)
		// redis 저장 실패를 vm생성 실패로 처리하지는 않음
	}

	// Redis 저장 완료 후 HTTP 전송 (Core에서 Redis 업데이트 가능)
	coreClient := client.NewCoreClient(selectedCore)
	_, err = coreClient.CreateVM(context.Background(), req)
	if err != nil {
		log.Error("Error creating VM on core %s: %v", selectedCore.IP, err, true)
		cleanup() // 직접 지우지 말고 요 함수 하나로--
		return fmt.Errorf("CreateVM: failed to create VM on core %s: %w", selectedCore.IP, err)
	}

	err = contextStruct.AddInstance(newVM, selectedCoreIndex)
	if err != nil {
		log.Error("Error database instance insertion failed: %v", err, true)
		cleanup() // 직접 지우지 말고 요 함수 하나로--
	}

	if newSubnetAllocated {
		_, err := contextStruct.DB.Exec("UPDATE subnet SET last_subnet = ? WHERE id = '1'", subnetReq.IP)
		if err != nil {
			log.Error("Error database Subnet insertion failed: %v", err, true)
		}
	}

	// VMLocation map과 AliveVM slice를 하나의 Lock으로 묶어 일관성 보장
	// (VMLocation에는 있는데 AliveVM에는 없는 중간 상태가 노출되지 않도록)
	contextStruct.Lock()
	if contextStruct.VMLocation == nil {
		contextStruct.VMLocation = make(map[vms.UUID]*vms.Core)
	}
	contextStruct.VMLocation[uuid] = &contextStruct.Cores[selectedCoreIndex]
	contextStruct.AliveVM = append(contextStruct.AliveVM, newVM)
	contextStruct.Unlock()
	log.Info("VM %s added to ControlContext", uuid, true)

	log.Info("UUID %s CreateVM request success on core %s", uuid, selectedCore.IP, true)
	return nil
}

func DeleteVM(uuid vms.UUID, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()
	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("VM with UUID %s not found", string(uuid))
		return fmt.Errorf("VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	_, err := coreClient.DeleteVM(context.Background(), model.DeleteVMRequest{
		UUID: uuid,
		Type: model.HardDelete,
	})
	if err != nil {
		log.Error("error deleting VM %s on core %s: %v", uuid, core.IP, err)
		return fmt.Errorf("DeleteVM: failed to delete VM %s on core %s: %w", uuid, core.IP, err)
	}

	err = contextStruct.DeleteInstance(uuid)
	if err != nil {
		log.Error("error deleting instance %s from ControlContext: %v", uuid, err)
		return fmt.Errorf("DeleteVM: failed to delete instance %s: %w", uuid, err)
	}
	if cleanupErr := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); cleanupErr != nil {
		log.Error("Failed to cleanup Guacamole config during rollback: %v", cleanupErr)
	}

	if err := RemoveVMInfoFromRedis(context.Background(), rdb, uuid); err != nil {
		log.Warn("failed to remove vm info from redis (vm deletion succeeded but..): %v", err, true)
		// 얘도 create할 때처럼 redis 값 삭제 실패를 했을 때 del vm 자체를 실패했다고 처리하지는 않음
		// 물론 어케 처리할지 고민을..
	}

	return nil
}
func StartVM(uuid vms.UUID, contextStruct *vms.ControlContext) error {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		return fmt.Errorf("VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	_, err := coreClient.StartVM(context.Background(), model.StartVMRequest{
		UUID: uuid,
	})
	if err != nil {
		return fmt.Errorf("StartVM: failed to start VM %s: %w", uuid, err)
	}

	log.Info("VM %s started on core %s", uuid, core.IP, true)
	return nil
}

func ShutdownVM(uuid vms.UUID, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		return fmt.Errorf("VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	_, err := coreClient.ForceShutdownVM(context.Background(), model.ForceShutdownVMRequest{
		UUID: uuid,
	})

	if err != nil {
		return fmt.Errorf("ShutdownVM: failed to shutdown VM %s: %w", uuid, err)
	}

	foundIndex := -1
	// 탐색과 삭제를 하나의 Lock 안에서 — 탐색 후 삭제 전에 다른 goroutine이 AliveVM을 바꾸면 index가 틀어짐
	contextStruct.Lock()
	for i, vm := range contextStruct.AliveVM {
		if vm.UUID == uuid {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		contextStruct.AliveVM = slices.Delete(contextStruct.AliveVM, foundIndex, foundIndex+1)
	}
	contextStruct.Unlock()

	if err := UpdateVMStatusInRedis(context.Background(), rdb, uuid, model.VMStatusStopped, time.Now().Unix()); err != nil {
		log := util.GetLogger()
		log.Warn("failed to update vm status in redis %v", err, true)
	}

	return nil
}

func GetVMCpuInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (model.CoreMachineCpuInfoResponse, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMCpuInfo: VM with UUID %s not found", string(uuid), true)
		return model.CoreMachineCpuInfoResponse{}, fmt.Errorf("GetVMCpuInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)

	cpuInfo, err := coreClient.GetVMCpuInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMCpuInfo: error getting CPU info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return model.CoreMachineCpuInfoResponse{}, fmt.Errorf("GetVMCpuInfo: error getting CPU info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved CPU status for VM %s on core %s", uuid, core.IP)
	return cpuInfo, nil
}

func GetVMMemoryInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (model.CoreMachineMemoryInfoResponse, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMMemoryInfo: VM with UUID %s not found", string(uuid), true)
		return model.CoreMachineMemoryInfoResponse{}, fmt.Errorf("GetVMMemoryInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)

	memoryInfo, err := coreClient.GetVMMemoryInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMMemoryInfo: error getting memory info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return model.CoreMachineMemoryInfoResponse{}, fmt.Errorf("GetVMMemoryInfo: error getting memory info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved Memory status for VM %s on core %s", uuid, core.IP)
	return memoryInfo, nil
}

func GetVMDiskInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (model.CoreMachineDiskInfoResponse, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMDiskInfo: VM with UUID %s not found", string(uuid), true)
		return model.CoreMachineDiskInfoResponse{}, fmt.Errorf("GetVMDiskInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)

	diskInfo, err := coreClient.GetVMDiskInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMDiskInfo: error getting disk info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return model.CoreMachineDiskInfoResponse{}, fmt.Errorf("GetVMDiskInfo: error getting disk info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved Disk status for VM %s on core %s", uuid, core.IP)
	return diskInfo, nil
}
