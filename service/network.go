package service

import (
	"fmt"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	pkgnetwork "github.com/easy-cloud-Knet/KWS_Control/pkg/network"
	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func AddCmsSubnet(ctx *vms.ControlContext, uuid vms.UUID) (*client.CmsResponse, error) {
	log := util.GetLogger()

	ip, err := GetVMIPByUUID(ctx, uuid)
	if err != nil {
		log.Error("AddCmsSubnet : GetVMIPByUUID: %w", err)
		return nil, err
	}
	subnet, err := pkgnetwork.GetSubnetFromIP(ip)
	if err != nil {
		log.Error("AddCmsSubnet : GetSubnetFromIP: %v", err)
		return nil, err
	}
	cmsClient := client.NewCmsClient()
	temp, err := cmsClient.CmsRequest(subnet)
	if err != nil {
		log.Error("AddCmsSubnet : CmsRequest(subnet): %v", err)
		return nil, err
	}

	return temp, nil
}

func NewCmsSubnet(ctx *vms.ControlContext) (*client.CmsResponse, error) {
	log := util.GetLogger()

	last_subnet := ctx.Last_subnet
	next_last_subnet := pkgnetwork.FindSubnet(last_subnet)
	log.Info("NewCmsSubnet : next_last_subnet: %s", next_last_subnet)

	cmsClient := client.NewCmsClient()
	temp, err := cmsClient.CmsRequest(next_last_subnet)
	if err != nil {
		log.Error("NewCmsSubnet : CmsRequest(subnet): %v", err)
		return nil, err
	}
	_, err = ctx.DB.Exec("UPDATE subnet SET last_subnet = ? WHERE id = 1", next_last_subnet)
	if err != nil {
		log.Error("Failed to update last_subnet in database: %v", err)
		return nil, err
	}
	ctx.Last_subnet = next_last_subnet
	return temp, nil
}

func GetVMIPByUUID(ctx *vms.ControlContext, uuid vms.UUID) (string, error) {
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
