package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

type CmsClient struct {
	baseURL string
	client  *http.Client
}

type CmsResponse struct {
	IP      string `json:"ip"`
	MacAddr string `json:"macAddr"`
	SdnUUID string `json:"sdnUUID"`
}

type CmsRequest struct {
	Subnet string `json:"Subnet"`
}

// fmt.Sprintf("%s/New/Instance", CMS_HOST)
func NewCmsClient() *CmsClient {
	CMS_HOST := os.Getenv("CMS_HOST")
	if CMS_HOST == "" {
		log := util.GetLogger()
		log.Error("CMS_HOST Re:Check your env variable", true)
		CMS_HOST = "localhost:8080"
		log.Warn("CMS_HOST set: %s", CMS_HOST, true)
	}
	return &CmsClient{
		baseURL: CMS_HOST,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *CmsClient) CmsRequest(Subnet string) (*CmsResponse, error) {
	log := util.GetLogger()

	req_url := fmt.Sprintf("http://%s/New/Instance", c.baseURL)
	reqBody := CmsRequest{Subnet: Subnet}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Error("CMS : failed to marshal JSON: %w", err)
		return nil, err
	}

	req, err := http.NewRequest("POST", req_url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Error("CMS : failed to NewRequest: %w", err)
		return nil, err
	}

	// Content-Type 헤더 설정
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.DebugInfo("Making request to: %s", req_url)
	log.DebugInfo("Request body: %s", string(jsonBody))

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("CMS : failed to create request: %w", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("CMS : CMS returned status: %s", resp.Status)
		return nil, fmt.Errorf("CMS server returned non-OK status: %s", resp.Status)
	}
	var addrResp CmsResponse
	if err := json.NewDecoder(resp.Body).Decode(&addrResp); err != nil {
		log.Error("CMS : failed to decode CMS response: %w", err)
		return nil, err
	}

	return &addrResp, nil
}

func (c *CmsClient) AddCmsSubnet(ctx *vms.ControlContext, uuid vms.UUID) (*CmsResponse, error) {
	log := util.GetLogger()

	ip, err := GetVMIPByUUID(ctx, uuid)
	if err != nil {
		log.Error("AddCmsSubnet : GetVMIPByUUID: %w", err)
		return nil, err
	}
	subnet, err := GetSubnetFromIP(ip)
	if err != nil {
		log.Error("AddCmsSubnet : GetSubnetFromIP: %v", err)
		return nil, err
	}
	temp, err := c.CmsRequest(subnet)
	if err != nil {
		log.Error("AddCmsSubnet : c.CmsRequest(subnet): %v", err)
		return nil, err
	}

	return temp, nil

}

func (c *CmsClient) NewCmsSubnet(ctx *vms.ControlContext) (*CmsResponse, error) {
	log := util.GetLogger()

	last_subnet := ctx.Last_subnet
	next_last_subnet := Find_subnet(last_subnet)
	log.Info("NewCmsSubnet : next_last_subnet: %s", next_last_subnet)

	temp, err := c.CmsRequest(next_last_subnet)
	if err != nil {
		log.Error("AddCmsSubnet : c.CmsRequest(subnet): %v", err)
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

func Find_subnet(last_subnet string) string {
	value := make([]int, 3)
	j := 0
	for i := 0; i < 3; i++ {
		var temp string
		for last_subnet[j] != '.' {
			temp = temp + string(last_subnet[j])
			j++
		}
		value[i], _ = strconv.Atoi(temp)
		j++
	}

	if value[2] >= 255 {
		if value[1] >= 255 {
			if value[0] >= 255 {
				return "err"
			} else {
				value[0]++
				value[1] = 0
				value[2] = 0
			}
		} else {
			value[1]++
			value[2] = 0
		}
	} else {
		value[2]++
	}

	result := fmt.Sprintf("%s.%s.%s.", strconv.Itoa(value[0]), strconv.Itoa(value[1]), strconv.Itoa(value[2]))
	return result
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

func GetSubnetFromIP(ip string) (string, error) {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid IP format: %s", ip)
	}

	return strings.Join(parts[:3], ".") + ".", nil
}
