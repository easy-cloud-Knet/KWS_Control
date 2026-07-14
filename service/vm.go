package service

import (
	"context"
	"fmt"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	"github.com/easy-cloud-Knet/KWS_Control/pkg/guacamole"
	internalssh "github.com/easy-cloud-Knet/KWS_Control/pkg/ssh"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"github.com/redis/go-redis/v9"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

func CreateVM(input CreateVMInput, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()
	uuid := input.UUID
	hwReq := vms.HardwareRequirement{
		Memory: input.HardwareInfo.Memory,
		CPU:    input.HardwareInfo.CPU,
		Disk:   input.HardwareInfo.Disk,
	}

	log.Info("func CreateVM() memory=%d GiB, cpu=%d, disk=%d GiB", hwReq.Memory, hwReq.CPU, hwReq.Disk, true)

	// 1) 코어 선택 + 자원 예약 (원자적: 선택과 Free* 차감이 한 락 안에서 → TOCTOU 제거)
	selectedCore, selectedCoreIndex, err := reserveCoreOrFail(contextStruct, hwReq)
	if err != nil {
		return err
	}

	// 예약 직후 롤백 등록. 이후 모든 실패 경로는 cleanup.run()으로 예약을 되돌려야 함
	cleanup := &cleanupChain{}
	cleanup.push(func() {
		contextStruct.Resources.DeallocateResources(selectedCore, uuid, hwReq)
	})

	// 2) SSH 키 생성
	privateKeyPEM, publicKeyOpenSSH, err := internalssh.GenerateSSHKey()
	if err != nil {
		cleanup.run()
		log.Error("GenerateSshKey() failed: %v", err, true)
		return fmt.Errorf("CreateVM: failed to generate SSH key: %w", err)
	}

	//사용자 수 확인
	if len(input.Users) == 0 {
		cleanup.run()
		log.Error("CreateVM: at least one user is required for Guacamole configuration", true)
		return fmt.Errorf("CreateVM: at least one user is required")
	}

	// 3) CMS 서브넷 할당 (Add: 기존 서브넷 / New: 신규 서브넷)
	cmsResp, isNewSubnet, err := allocateCmsSubnet(contextStruct, input.SubnetType, uuid)
	if err != nil {
		//TODO  AllocateCmsSubnet cleanup logic implement
		cleanup.run()
		log.Error("CreateVM: failed to allocate CMS subnet: %v", err, true)
		return fmt.Errorf("CreateVM: failed to allocate CMS subnet: %w", err)
	}

	log.DebugInfo("CMS allocated: ip=%s, mac=%s, sdn=%s", cmsResp.IP, cmsResp.MacAddr, cmsResp.SdnUUID)

	// 4) Guacamole 사용자/커넥션 설정
	userPass := guacamole.Configure(input.Users[0].Name, string(uuid), cmsResp.IP, privateKeyPEM, contextStruct.GuacDB)
	if userPass == "" {
		cleanup.run()
		log.Error("CreateVM: failed to configure Guacamole", true)
		return fmt.Errorf("CreateVM: failed to configure Guacamole")
	}
	cleanup.push(func() {
		log.Info("clean up: guacamole")
		if err := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); err != nil {
			log.Error("Failed to cleanup Guacamole config during rollback: %v", err)
		}
	})

	newVM := &vms.VMInfo{
		UUID:         uuid,
		GuacPassword: userPass,
		MacAddr:      cmsResp.MacAddr,
		Memory:       hwReq.Memory,
		Cpu:          hwReq.CPU,
		Disk:         hwReq.Disk,
		IP_VM:        cmsResp.IP,
	}

	// 5) 코어에 VM 메타데이터 부착 (Free* 예약은 ReserveCore에서 완료, 롤백은 위 cleanup이 담당)
	contextStruct.Resources.AttachVMInfo(selectedCore, uuid, newVM)
	log.DebugInfo("core %s updated: FreeMemory=%d, FreeCPU=%d, FreeDisk=%d",
		selectedCore.IP, selectedCore.FreeMemory, selectedCore.FreeCPU, selectedCore.FreeDisk)

	// 6) Redis에 초기 상태 저장 (Core 호출 전에 — Core가 상태 갱신할 수 있도록)
	if err := StoreVMInfoToRedis(context.Background(), rdb, VMInfo{
		UUID:   uuid,
		CPU:    hwReq.CPU,
		Memory: hwReq.Memory,
		Disk:   hwReq.Disk,
		IP:     cmsResp.IP,
		Status: VMStatusUnknown,
	}, time.Now().Unix()); err != nil {
		log.Warn("failed to store VM info to redis: %v", err, true)
		// redis 저장 실패는 vm 생성 실패로 처리하지 않음
	}

	// 7) Core에 VM 생성 요청 — service DTO를 client contract로 변환
	coreReq := buildCoreCreateVMRequest(input, cmsResp, publicKeyOpenSSH)
	coreClient := client.NewCoreClient(selectedCore)
	if _, err := coreClient.CreateVM(context.Background(), coreReq); err != nil {
		log.Error("Error creating VM on core %s: %v", selectedCore.IP, err, true)
		cleanup.run()
		return fmt.Errorf("CreateVM: failed to create VM on core %s: %w", selectedCore.IP, err)
	}

	// 코어 생성 성공 이후의 실패 발생 시 처리
	cleanup.push(func() {
		log.Info("clean up: requesting VM deletion on core %s for %s", selectedCore.IP, uuid, true)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := coreClient.DeleteVM(ctx, model.DeleteVMRequest{
			UUID: uuid,
			Type: model.HardDelete,
		}); err != nil {
			// 삭제 실패 자체가 롤백을 중단시키지 않도록.
			log.Error("rollback: failed to delete VM %s on core %s (manual cleanup or core GC required): %v",
				uuid, selectedCore.IP, err, true)
		}
		if err := RemoveVMInfoFromRedis(ctx, rdb, uuid); err != nil {
			log.Warn("rollback: failed to remove vm info from redis: %v", err, true)
		}
	})

	// 8) DB에 인스턴스 정보 영속화
	if err := contextStruct.VMRepo.AddInstance(newVM, selectedCoreIndex); err != nil {
		log.Error("Error database instance insertion failed: %v", err, true)
		cleanup.run()
		return fmt.Errorf("CreateVM: failed to persist instance: %w", err)
	}

	// 8-1) 신규 서브넷 할당이었다면 last_subnet을 실제 할당된 IP로 갱신
	// (NewCmsSubnet에서 계산값으로 선점한 것을 CMS 응답값으로 정정)
	if isNewSubnet {
		if _, err := contextStruct.DB.Exec("UPDATE subnet SET last_subnet = ? WHERE id = '1'", cmsResp.IP); err != nil {
			log.Error("Error database Subnet update failed: %v", err, true)
		}
	}

	contextStruct.Resources.RegisterVM(uuid, &contextStruct.Resources.Cores[selectedCoreIndex], newVM)
	log.Info("VM %s added to ControlContext", uuid, true)

	log.Info("UUID %s CreateVM request success on core %s", uuid, selectedCore.IP, true)
	return nil
}

// 추후 Mapper 계층이 필요할 것으로 예상됨
func buildCoreCreateVMRequest(input CreateVMInput, cmsResp *client.CmsNewInstanceResponse, publicKeyOpenSSH string) model.CreateVMRequest {
	users := make([]model.UserInfoVM, len(input.Users))
	for i, u := range input.Users {
		users[i] = model.UserInfoVM{
			Name:              u.Name,
			Groups:            u.Groups,
			Password:          u.Password,
			SSHAuthorizedKeys: u.SSHAuthorizedKeys,
		}
	}
	// 첫 번째 사용자는 생성 시 발급한 SSH 공개키를 사용
	if len(users) > 0 {
		users[0].SSHAuthorizedKeys = []string{publicKeyOpenSSH}
	}
	return model.CreateVMRequest{
		DomType:      input.DomType,
		DomName:      input.DomName,
		UUID:         input.UUID,
		OS:           input.OS,
		HardwareInfo: model.HardwareInfo{CPU: input.HardwareInfo.CPU, Memory: input.HardwareInfo.Memory, Disk: input.HardwareInfo.Disk},
		NetConf:      model.NetDefine{Ips: []string{cmsResp.IP}, NetType: 0},
		Users:        users,
		SdnUUID:      cmsResp.SdnUUID,
		MacAddr:      cmsResp.MacAddr,
		Subnettype:   input.SubnetType,
	}
}

// reserveCoreOrFail은 코어 선택+예약 + 실패 시 진단 로그 출력 캡슐화를 진행.
// 성공 반환 시 해당 코어의 Free* 자원은 이미 예약(차감)된 상태 - 호출자는 실패 경로에서 롤백해야 함.
func reserveCoreOrFail(contextStruct *vms.ControlContext, req vms.HardwareRequirement) (*vms.Core, int, error) {
	log := util.GetLogger()

	log.DebugInfo("core selection process. req: memory=%d GiB, cpu=%d, disk=%d", req.Memory, req.CPU, req.Disk)
	result := contextStruct.Resources.ReserveCore(req)

	if result.Core != nil {
		log.DebugInfo("core found: %s (idx=%d)", result.Core.IP, result.Index)
		return result.Core, result.Index, nil
	}

	log.Error("No suitable core found! Total cores: %d, Alive cores: %d, Required: Memory=%d CPU=%d Disk=%d",
		result.TotalCores, result.AliveCount, req.Memory, req.CPU, req.Disk, true)

	if result.AliveCount > 0 {
		log.DebugError("alive cores:")
		contextStruct.Resources.RLock()
		for i := range contextStruct.Resources.Cores {
			core := &contextStruct.Resources.Cores[i]
			if core.IsAlive {
				log.DebugError("  %s: Memory=%d/%d, CPU=%d/%d, Disk=%d/%d",
					core.IP, core.FreeMemory, core.CoreInfoIdx.Memory,
					core.FreeCPU, core.CoreInfoIdx.Cpu,
					core.FreeDisk, core.CoreInfoIdx.Disk)
			}
		}
		contextStruct.Resources.RUnlock()
	} else {
		log.DebugError("no alive cores available")
	}

	return nil, -1, fmt.Errorf("CreateVM: no suitable core found")
}

func allocateCmsSubnet(contextStruct *vms.ControlContext, subnetType string, uuid vms.UUID) (*client.CmsNewInstanceResponse, bool, error) {
	log := util.GetLogger()
	cmsClient := client.NewCmsClient()

	if subnetType == "Add" {
		resp, err := AddCmsSubnet(cmsClient, contextStruct, uuid)
		if err != nil {
			log.Error("CreateVM: failed to configure cms (Add): %v", err, true)
			return nil, false, fmt.Errorf("CreateVM: failed to configure cms: %w", err)
		}
		return resp, false, nil
	}

	resp, err := NewCmsSubnet(cmsClient, contextStruct)
	if err != nil {
		log.Error("CreateVM: failed to configure cms (New): %v", err, true)
		return nil, false, fmt.Errorf("CreateVM: failed to configure cms: %w", err)
	}
	// 신규 서브넷 할당 시: 후속 단계 실패에 대한 별도 DB 롤백 로직은 미구현 상태 (TODO)

	return resp, true, nil
}

func DeleteVM(uuid vms.UUID, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("VM with UUID %s not found", string(uuid))
		return fmt.Errorf("VM with UUID %s not found", string(uuid))
	}

	// 1) Core에 VM 삭제 요청
	coreClient := client.NewCoreClient(core)
	if _, err := coreClient.DeleteVM(context.Background(), model.DeleteVMRequest{
		UUID: uuid,
		Type: model.HardDelete,
	}); err != nil {
		log.Error("error deleting VM %s on core %s: %v", uuid, core.IP, err)
		return fmt.Errorf("DeleteVM: failed to delete VM %s on core %s: %w", uuid, core.IP, err)
	}

	// Core 삭제 성공 -> 인메모리 자원 회수 (Free* 복구 + VMInfoIdx/VMLocation/AliveVM 정리)
	if contextStruct.Resources.ReleaseVM(core, uuid) {
		log.DebugInfo("released in-memory resources for VM %s on core %s: FreeMemory=%d, FreeCPU=%d, FreeDisk=%d",
			uuid, core.IP, core.FreeMemory, core.FreeCPU, core.FreeDisk)
	}

	cmsClient := client.NewCmsClient()
	if err := DeleteCmsSubnet(cmsClient, contextStruct, uuid); err != nil {
		log.Error("DeleteVM: failed to delete CMS subnet for VM %s: %v", uuid, err)
		// CMS 삭제 실패는 로그만 남기고 삭제 자체는 성공으로 처리 (추가적인 수동 정리 필요)
	}

	// 2) DB에서 인스턴스 정보 삭제
	if err := contextStruct.VMRepo.DeleteInstance(uuid); err != nil {
		log.Error("error deleting instance %s from DB: %v", uuid, err)
		return fmt.Errorf("DeleteVM: failed to delete instance %s: %w", uuid, err)
	}

	// 3) Guacamole 사용자/커넥션 정리
	if err := guacamole.Cleanup(string(uuid), contextStruct.GuacDB); err != nil {
		log.Error("Failed to cleanup Guacamole config: %v", err)
	}

	// 4) Redis 정리 (실패해도 삭제 자체는 성공으로 처리 추가 처리 로직 필요)
	if err := RemoveVMInfoFromRedis(context.Background(), rdb, uuid); err != nil {
		log.Warn("failed to remove vm info from redis (vm deletion succeeded but..): %v", err, true)
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
	if _, err := coreClient.StartVM(context.Background(), model.StartVMRequest{UUID: uuid}); err != nil {
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
	if _, err := coreClient.ForceShutdownVM(context.Background(), model.ForceShutdownVMRequest{UUID: uuid}); err != nil {
		return fmt.Errorf("ShutdownVM: failed to shutdown VM %s: %w", uuid, err)
	}

	contextStruct.Resources.UnregisterAlive(uuid)

	if err := UpdateVMStatusInRedis(context.Background(), rdb, uuid, VMStatusStopped, time.Now().Unix()); err != nil {
		util.GetLogger().Warn("failed to update vm status in redis %v", err, true)
	}

	return nil
}

func GetVMCpuInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (VMCpuStatus, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMCpuInfo: VM with UUID %s not found", string(uuid), true)
		return VMCpuStatus{}, fmt.Errorf("GetVMCpuInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	cpuInfo, err := coreClient.GetVMCpuInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMCpuInfo: error getting CPU info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return VMCpuStatus{}, fmt.Errorf("GetVMCpuInfo: error getting CPU info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved CPU status for VM %s on core %s", uuid, core.IP)
	return VMCpuStatus{System: cpuInfo.System, Idle: cpuInfo.Idle, Usage: cpuInfo.Usage}, nil
}

func GetVMMemoryInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (VMMemoryStatus, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMMemoryInfo: VM with UUID %s not found", string(uuid), true)
		return VMMemoryStatus{}, fmt.Errorf("GetVMMemoryInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	memoryInfo, err := coreClient.GetVMMemoryInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMMemoryInfo: error getting memory info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return VMMemoryStatus{}, fmt.Errorf("GetVMMemoryInfo: error getting memory info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved Memory status for VM %s on core %s", uuid, core.IP)
	return VMMemoryStatus{
		Total:       memoryInfo.Total,
		Used:        memoryInfo.Used,
		Available:   memoryInfo.Available,
		UsedPercent: memoryInfo.UsedPercent,
	}, nil
}

func GetVMDiskInfo(uuid vms.UUID, contextStruct *vms.ControlContext) (VMDiskStatus, error) {
	log := util.GetLogger()

	core := contextStruct.FindCoreByVmUUID(uuid)
	if core == nil {
		log.Error("GetVMDiskInfo: VM with UUID %s not found", string(uuid), true)
		return VMDiskStatus{}, fmt.Errorf("GetVMDiskInfo: VM with UUID %s not found", string(uuid))
	}

	coreClient := client.NewCoreClient(core)
	diskInfo, err := coreClient.GetVMDiskInfo(context.Background(), uuid)
	if err != nil {
		log.Error("GetVMDiskInfo: error getting disk info for VM %s on core %s: %v", uuid, core.IP, err, true)
		return VMDiskStatus{}, fmt.Errorf("GetVMDiskInfo: error getting disk info for VM %s on core %s: %w", uuid, core.IP, err)
	}

	log.DebugInfo("Retrieved Disk status for VM %s on core %s", uuid, core.IP)
	return VMDiskStatus{
		Total:       diskInfo.Total,
		Used:        diskInfo.Used,
		Free:        diskInfo.Free,
		UsedPercent: diskInfo.UsedPercent,
	}, nil
}
