package service

import (
	"fmt"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	pkgnetwork "github.com/easy-cloud-Knet/KWS_Control/pkg/network"
	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func AddCmsSubnet(c *client.SubnetClient, ctx *vms.ControlContext, uuid vms.UUID) (*client.NewSubnetRequest, error) {
	log := util.GetLogger()

	ip, err := GetVMIPByUUID(ctx, uuid)
	if err != nil {
		log.Error("AddCmsSubnet : GetVMIPByUUID: %v", err)
		return nil, fmt.Errorf("AddCmsSubnet: failed to get VM IP: %w", err)
	}
	subnet, err := pkgnetwork.GetSubnetFromIP(ip)
	if err != nil {
		log.Error("AddCmsSubnet : GetSubnetFromIP: %v", err)
		return nil, fmt.Errorf("AddCmsSubnet: failed to get subnet: %w", err)
	}
	resp, err := c.RequestSubnet(subnet)
	if err != nil {
		log.Error("AddCmsSubnet : RequestSubnet: %v", err)
		return nil, fmt.Errorf("AddCmsSubnet: RequestSubnet failed: %w", err)
	}

	return resp, nil
}

func NewCmsSubnet(c *client.SubnetClient, ctx *vms.ControlContext) (*client.NewSubnetRequest, error) {
	log := util.GetLogger()

	last_subnet := ctx.Last_subnet
	next_last_subnet := pkgnetwork.FindSubnet(last_subnet)
	log.Info("NewCmsSubnet : next_last_subnet: %s", next_last_subnet)

	// DB를 먼저 업데이트하여 서브넷을 선점한다.
	// CMS 호출 전에 선점해야 실패 시 동일 서브넷이 중복 할당되는 것을 방지할 수 있다.
	_, err := ctx.DB.Exec("UPDATE subnet SET last_subnet = ? WHERE id = 1", next_last_subnet)
	if err != nil {
		log.Error("Failed to update last_subnet in database: %v", err)
		return nil, fmt.Errorf("NewCmsSubnet: failed to update last_subnet in DB: %w", err)
	}
	ctx.Last_subnet = next_last_subnet

	resp, err := c.RequestSubnet(next_last_subnet)
	if err != nil {
		log.Error("NewCmsSubnet : RequestSubnet: %v", err)
		// CMS 호출 실패 시 DB를 원래 값으로 롤백
		if _, rbErr := ctx.DB.Exec("UPDATE subnet SET last_subnet = ? WHERE id = 1", last_subnet); rbErr != nil {
			log.Error("NewCmsSubnet : failed to rollback last_subnet: %v", rbErr)
			return nil, fmt.Errorf("NewCmsSubnet: RequestSubnet failed: %w, rollback also failed: %v", err, rbErr)
		}
		ctx.Last_subnet = last_subnet
		return nil, fmt.Errorf("NewCmsSubnet: RequestSubnet failed: %w", err)
	}

	return resp, nil
}

func GetVMIPByUUID(ctx *vms.ControlContext, uuid vms.UUID) (string, error) {
	ctx.RLock()
	defer ctx.RUnlock()

	core, ok := ctx.VMLocation[uuid]
	if !ok {
		return "", fmt.Errorf("UUID %s not found in VMLocation", uuid)
	}

	vmInfo, ok := core.VMInfoIdx[uuid]
	if !ok {
		return "", fmt.Errorf("VMInfo for UUID %s not found in Core", uuid)
	}

	return vmInfo.IP_VM, nil
}
