package structure

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/util"
)

type ControlContext struct {
	mu          sync.RWMutex
	Config      Config
	DB          *sql.DB
	GuacDB      *sql.DB
	Last_subnet string
	Cores       []Core         // 모든 코어를 관리
	AliveVM     []*VMInfo      //현재 가동중인 VM의 정보
	VMLocation  map[UUID]*Core //UUID를 기반으로 어떤 VM이 어느 Core에 있는지 확인하는 포인터
	// => 여기에다가는 사실 core에 ip주소를 기반으로 어느 ip에 어느 UUID를 가진 VM이 있는지만 확인할 수 있으면 될거같아요.
	// => 근데 이거 자료형을 어떤걸 써야할지 모르겠어서 일단 이렇게 map[UUID]*Core로 해놓은거지 2차원 벡터로 해도 무방할거 같습니다.
	// => 이렇게 한 이유는 이전 버전에서 VMPool map[UUID]*VM 이렇게 해놓았던데 이렇게 하면 VM이 많아졌을 때 너무 오래 걸릴거 같아서 IP를 기반으로 먼저 찾고 해당 core로 넘어가서 처리하면 더 좋지않을까? 생각했어요.
}

func (c *ControlContext) Lock()    { c.mu.Lock() }
func (c *ControlContext) Unlock()  { c.mu.Unlock() }
func (c *ControlContext) RLock()   { c.mu.RLock() }
func (c *ControlContext) RUnlock() { c.mu.RUnlock() }

func (c *ControlContext) FindCoreByVmUUID(uuid UUID) *Core {
	log := util.GetLogger()

	coreIdx, err := c.GetInstanceLocation(uuid)
	if err != nil {
		log.Error("Core not found for VM UUID %s", uuid, true)
		return nil
	}
	return &c.Cores[coreIdx]
}

func (contextStructure *ControlContext) AddInstance(instanceInfo *VMInfo, coreIdx int) error {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction %v", err)
		return err
	}
	defer tx.Rollback()

	// Set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tx.ExecContext(ctx, "INSERT INTO inst_info (uuid, inst_ip, guac_pass, inst_mem, inst_vcpu, inst_disk) VALUES (?, ?, ?, ?, ?, ?)",
		string(instanceInfo.UUID),
		instanceInfo.IP_VM,
		instanceInfo.GuacPassword,
		instanceInfo.Memory,
		instanceInfo.Cpu,
		instanceInfo.Disk)
	if err != nil {
		log.Error("Failed to insert instance info: %v", err)
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO inst_loc (uuid, core) VALUES (?, ?)",
		string(instanceInfo.UUID),
		coreIdx)
	if err != nil {
		log.Error("Failed to insert instance core relation: %v", err)
		return err
	}
	return tx.Commit()
}

// 현재 미사용중
// 이건 미사용중이면 안되지않나? 인스턴스정보 DB에 업데이트 하는거같은데
func (contextStructure *ControlContext) UpdateInstance(instanceInfo *VMInfo) error {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction %v", err)
		return err
	}
	defer tx.Rollback()

	//Set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = tx.ExecContext(ctx, "UPDATE inst_info SET inst_ip = ?, inst_mem = ?, inst_vcpu = ?, inst_disk = ? WHERE uuid = ?",
		instanceInfo.IP_VM,
		instanceInfo.Memory,
		instanceInfo.Cpu,
		instanceInfo.Disk,
		string(instanceInfo.UUID))
	if err != nil {
		log.Error("Failed to update instance info: %v", err)
		return err
	}
	return tx.Commit()
}

func (contextStructure *ControlContext) DeleteInstance(uuid UUID) error {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction: %v", err)
		return err
	}
	defer tx.Rollback()
	// Set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tx.ExecContext(ctx, "DELETE FROM inst_info WHERE uuid = ?", uuid)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM inst_loc WHERE uuid = ?", uuid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (contextStructure *ControlContext) GetInstance(uuid UUID) (*VMInfo, error) {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction: %v", err)
		return nil, err
	}
	defer tx.Rollback()
	//set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var instance VMInfo
	err = tx.QueryRowContext(ctx, "SELECT uuid, inst_ip, guac_pass, inst_mem, inst_vcpu, inst_disk FROM inst_info WHERE uuid = ?", uuid).Scan(
		&instance.UUID,
		&instance.IP_VM,
		&instance.GuacPassword,
		&instance.Memory,
		&instance.Cpu,
		&instance.Disk)
	if err != nil {
		log.Error("Failed to get instance info: %v", err)
		return nil, err
	}
	return &instance, tx.Commit()
}

func (contextStructure *ControlContext) GetInstanceLocation(uuid UUID) (int, error) {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction: %v", err)
		return 0, err
	}
	defer tx.Rollback()
	//set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var coreIdx int
	err = tx.QueryRowContext(ctx, "SELECT core FROM inst_loc WHERE uuid = ?", uuid).Scan(&coreIdx)
	if err != nil {
		log.Error("Failed to get instance location: %v", err)
		return 0, err
	}
	return coreIdx, tx.Commit()
}

func (contextStructure *ControlContext) GetAllInstanceInfo() ([]VMInfo, []int, error) {
	log := util.GetLogger()
	tx, err := contextStructure.DB.Begin()
	if err != nil {
		log.Error("Failed to start transaction: %v", err)
		return nil, nil, err
	}
	defer tx.Rollback()
	//set timeout for the transaction
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var rows *sql.Rows
	rows, err = tx.QueryContext(ctx, "SELECT info.uuid, loc.core, info.inst_ip, info.guac_pass, info.inst_vcpu, info.inst_mem, info.inst_disk FROM inst_loc loc JOIN inst_info info ON loc.uuid = info.uuid")
	if err != nil {
		log.Error("Failed to get joined instance info: %v", err)
		return nil, nil, err
	}

	var coreIdxList []int
	var VMInfoList []VMInfo

	for rows.Next() {
		var coreIdx int
		var info VMInfo

		if err := rows.Scan(&info.UUID, &coreIdx, &info.IP_VM, &info.GuacPassword, &info.Cpu, &info.Memory, &info.Disk); err != nil {
			log.Error("Failed to scan instance location: %v", err)
			return nil, nil, err
		}
		log.DebugInfo("Found instance: %s on core %d", info.UUID, coreIdx)
		VMInfoList = append(VMInfoList, info)
		coreIdxList = append(coreIdxList, coreIdx)
	}
	return VMInfoList, coreIdxList, tx.Commit()
}
