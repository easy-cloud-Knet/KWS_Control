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
// 하드 필터+코사인으로 적합 코어를 고르고(SelectCore), alloc-Redis에 예약한 뒤
// CMS/Guacamole/Core에 VM을 생성한다. 실패 시 cleanup()이 예약을 롤백한다.
func CreateVM(req model.CreateVMRequest, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()
	ctx := context.Background()

	log.Info("func CreateVM() memory=%d MiB, cpu=%d, disk=%d MiB", req.HardwareInfo.Memory, req.HardwareInfo.CPU, req.HardwareInfo.Disk, true)

	// Guacamole 설정과 SSH 키 주입에 최소 1명의 사용자가 필요 — req.Users[0] 인덱싱 전에 방어적으로 검증.
	// (API 핸들러에서도 400으로 막지만, 서비스 직접 호출 대비.)
	if len(req.Users) == 0 {
		log.Error("CreateVM: at least one user is required", true)
		return fmt.Errorf("CreateVM: at least one user is required")
	}

	// SSH 키는 예약 전에 생성 — 실패해도 롤백할 예약이 없도록.
	privateKeyPEM, publicKeyOpenSSH, err := internalssh.GenerateSSHKey()
	if err != nil {
		log.Error("GenerateSshKey() failed: %v", err, true)
		return fmt.Errorf("CreateVM: failed to generate SSH key: %w", err)
	}

	uuid := req.UUID

	// ---- 코어 선택 + 예약 (단일 임계구역) ----
	// Lock → SelectCore → IncrCoreAlloc(+) → Unlock 을 한 임계구역으로 묶어,
	// 두 CreateVM이 같은 할당값을 읽고 같은 코어를 더블 부킹하는 것을 막는다.
	// HINCRBY 원자성만으론 read-then-decide가 안전하지 않으므로 인메모리 mutex가 직렬화 지점.
	// (단일 Control 프로세스 가정 — 수평 확장 시 Lua CAS로 교체.)
	// 느린 I/O(CMS/Guacamole/Core /createVM)는 락을 잡지 않고 임계구역 밖에서 수행.
	contextStruct.Lock()
	selectedCore, selectedCoreIndex, selErr := SelectCore(ctx, req.HardwareInfo, contextStruct, rdb)
	if selErr != nil {
		contextStruct.Unlock()
		log.Error("CreateVM: %v", selErr, true)
		return fmt.Errorf("CreateVM: %w", selErr)
	}
	if reserveErr := IncrCoreAlloc(ctx, rdb, selectedCore.IP, selectedCore.Port,
		int64(req.HardwareInfo.CPU), int64(req.HardwareInfo.Memory), int64(req.HardwareInfo.Disk)); reserveErr != nil {
		contextStruct.Unlock()
		log.Error("CreateVM: failed to reserve alloc on core %s:%d: %v", selectedCore.IP, selectedCore.Port, reserveErr, true)
		return fmt.Errorf("CreateVM: failed to reserve allocation: %w", reserveErr)
	}
	contextStruct.Unlock()

	// 롤백 플래그
	allocReserved := true        // alloc-Redis 예약 완료(IncrCoreAlloc+)
	guacamoleConfigured := false // Guacamole 설정 완료
	vmTracked := false           // VMInfoIdx 등록 + Free* 표시 캐시 반영 완료
	newSubnetAllocated := false

	cleanup := func() {
		if guacamoleConfigured {
			log.Info("clean up clean up")
			if cleanupErr := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); cleanupErr != nil {
				log.Error("Failed to cleanup Guacamole config during rollback: %v", cleanupErr)
			}
		}
		if vmTracked {
			// VMInfoIdx와 Free* 표시 캐시는 여러 핸들러가 동시에 접근하므로 Lock 필요
			contextStruct.Lock()
			delete(selectedCore.VMInfoIdx, uuid)
			selectedCore.FreeMemory += req.HardwareInfo.Memory
			selectedCore.FreeCPU += req.HardwareInfo.CPU
			selectedCore.FreeDisk += req.HardwareInfo.Disk
			contextStruct.Unlock()
		}
		if allocReserved {
			// 예약을 감산해 alloc-Redis를 이전 합으로 되돌린다.
			if relErr := IncrCoreAlloc(ctx, rdb, selectedCore.IP, selectedCore.Port,
				-int64(req.HardwareInfo.CPU), -int64(req.HardwareInfo.Memory), -int64(req.HardwareInfo.Disk)); relErr != nil {
				log.Error("Failed to release alloc reservation during rollback on core %s:%d: %v", selectedCore.IP, selectedCore.Port, relErr)
			}
			allocReserved = false
		}
		if newSubnetAllocated {
			//subnet--
		}
	}

	// add : back -> vm uuid    ->  cms   다른 api
	// new : subnet 찾기 -> cms
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
		cleanup()
		return fmt.Errorf("CreateVM: failed to configure cms: %w", err)
	}

	log.DebugInfo("CMS allocated: ip=%s, mac=%s, sdn=%s", subnetReq.IP, subnetReq.MacAddr, subnetReq.SdnUUID)

	userPass := guacamole.Configure(req.Users[0].Name, string(uuid), subnetReq.IP, privateKeyPEM, contextStruct.GuacDB)

	if userPass == "" {
		log.Error("CreateVM: failed to configure Guacamole", true)
		cleanup()
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

	// VMInfoIdx 등록 + Free* 표시 캐시 감소(자원 회계의 진실 소스는 위 IncrCoreAlloc).
	contextStruct.Lock()
	if selectedCore.VMInfoIdx == nil {
		selectedCore.VMInfoIdx = make(map[vms.UUID]*vms.VMInfo)
	}
	selectedCore.VMInfoIdx[uuid] = newVM
	selectedCore.FreeMemory -= req.HardwareInfo.Memory
	selectedCore.FreeCPU -= req.HardwareInfo.CPU
	selectedCore.FreeDisk -= req.HardwareInfo.Disk
	vmTracked = true
	contextStruct.Unlock()
	// 이후 HTTP 전송은 락 밖에서 — 네트워크 콜 중 락을 잡으면 다른 요청 전체가 블로킹됨

	log.DebugInfo("core %s reserved alloc+ cpu=%d mem=%d disk=%d", selectedCore.IP, req.HardwareInfo.CPU, req.HardwareInfo.Memory, req.HardwareInfo.Disk)

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
	if err := StoreVMInfoToRedis(ctx, rdb, vmRedisInfo); err != nil {
		log.Warn("failed to store VM info to redis: %v", err, true)
		// redis 저장 실패를 vm생성 실패로 처리하지는 않음
	}

	// Redis 저장 완료 후 HTTP 전송 (Core에서 Redis 업데이트 가능)
	coreClient := client.NewCoreClient(selectedCore)
	_, err = coreClient.CreateVM(ctx, req)
	if err != nil {
		log.Error("Error creating VM on core %s: %v", selectedCore.IP, err, true)
		cleanup() // 직접 지우지 말고 요 함수 하나로--
		return fmt.Errorf("CreateVM: failed to create VM on core %s: %w", selectedCore.IP, err)
	}

	err = contextStruct.AddInstance(newVM, selectedCoreIndex)
	if err != nil {
		log.Error("Error database instance insertion failed: %v", err, true)
		cleanup() // 직접 지우지 말고 요 함수 하나로--
		return fmt.Errorf("CreateVM: failed to persist instance %s: %w", uuid, err)
	}

	// last_subnet 영속화는 NewCmsSubnet의 선점 단계에서 단일 처리 — 여기서 IP로 다시 덮어쓰지 않는다.
	// (FindSubnet은 앞 3옥텟만 파싱하므로 subnet/IP 입력의 결과가 동일하고, 인메모리·DB 일관성을 확보.)

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
	ctx := context.Background()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("VM with UUID %s not found", string(uuid))
		return fmt.Errorf("VM with UUID %s not found", string(uuid))
	}

	// 삭제 전에 인메모리 VMInfoIdx에서 회수할 자원 크기와 VM IP를 확보.
	// (IP는 CMS 해제에 필요 — 아래 인메모리 정리 전에 캡처해 둬야 한다.)
	contextStruct.RLock()
	var dCPU, dMem, dDisk int64
	var vmIP string
	sizeKnown := false
	if vmInfo, ok := core.VMInfoIdx[uuid]; ok {
		dCPU, dMem, dDisk = int64(vmInfo.Cpu), int64(vmInfo.Memory), int64(vmInfo.Disk)
		vmIP = vmInfo.IP_VM
		sizeKnown = true
	}
	contextStruct.RUnlock()

	coreClient := client.NewCoreClient(core)
	_, err := coreClient.DeleteVM(ctx, model.DeleteVMRequest{
		UUID: uuid,
		Type: model.HardDelete,
	})
	if err != nil {
		log.Error("error deleting VM %s on core %s: %v", uuid, core.IP, err)
		return fmt.Errorf("DeleteVM: failed to delete VM %s on core %s: %w", uuid, core.IP, err)
	}

	// CMS 측 할당(IP/MAC/SDN) 해제 — 누락 시 CMS 풀 누수. 인메모리 정리 전에 캡처한 vmIP 사용.
	// 실패는 (guac/redis 정리와 동일 정책) 경고만 남기고 삭제 자체는 성공으로 처리한다.
	if vmIP != "" {
		cmsClient := client.NewCmsClient()
		if _, cmsErr := cmsClient.RequestDeleteInstance(vmIP); cmsErr != nil {
			log.Warn("DeleteVM: failed to release CMS allocation for %s (ip=%s): %v", uuid, vmIP, cmsErr, true)
		}
	} else {
		log.Warn("DeleteVM: IP for %s unknown (not in VMInfoIdx); CMS allocation not released", uuid, true)
	}

	err = contextStruct.DeleteInstance(uuid)
	if err != nil {
		log.Error("error deleting instance %s from ControlContext: %v", uuid, err)
		return fmt.Errorf("DeleteVM: failed to delete instance %s: %w", uuid, err)
	}
	if cleanupErr := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); cleanupErr != nil {
		log.Error("Failed to cleanup Guacamole config during rollback: %v", cleanupErr)
	}

	if err := RemoveVMInfoFromRedis(ctx, rdb, uuid); err != nil {
		log.Warn("failed to remove vm info from redis (vm deletion succeeded but..): %v", err, true)
		// 얘도 create할 때처럼 redis 값 삭제 실패를 했을 때 del vm 자체를 실패했다고 처리하지는 않음
		// 물론 어케 처리할지 고민을..
	}

	// Core + DB 삭제 성공 후 alloc-Redis 회수(회수된 용량 즉시 재사용 가능).
	if sizeKnown {
		if err := IncrCoreAlloc(ctx, rdb, core.IP, core.Port, -dCPU, -dMem, -dDisk); err != nil {
			log.Warn("DeleteVM: failed to release alloc for %s on core %s:%d: %v", uuid, core.IP, core.Port, err, true)
		}
	} else {
		log.Warn("DeleteVM: size for %s unknown (not in VMInfoIdx); alloc not adjusted, rebuild on restart will correct", uuid, true)
	}

	// 인메모리 상태 정리(기존 누락 갭 보완): VMInfoIdx / VMLocation / AliveVM + Free* 표시 캐시 복원.
	contextStruct.Lock()
	delete(core.VMInfoIdx, uuid)
	delete(contextStruct.VMLocation, uuid)
	for i, vm := range contextStruct.AliveVM {
		if vm.UUID == uuid {
			contextStruct.AliveVM = slices.Delete(contextStruct.AliveVM, i, i+1)
			break
		}
	}
	if sizeKnown {
		core.FreeMemory += uint32(dMem)
		core.FreeCPU += uint32(dCPU)
		core.FreeDisk += uint32(dDisk)
	}
	contextStruct.Unlock()

	log.Info("VM %s deleted from core %s", uuid, core.IP, true)
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
