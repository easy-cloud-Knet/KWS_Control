package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

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

func (c *CmsClient) RequestSubnet(subnet string) (*CmsResponse, error) {
	log := util.GetLogger()

	reqURL := fmt.Sprintf("http://%s/New/Instance", c.baseURL)
	reqBody := CmsRequest{Subnet: subnet}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Error("CMS : failed to marshal JSON: %v", err)
		return nil, fmt.Errorf("RequestSubnet: failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Error("CMS : failed to NewRequest: %v", err)
		return nil, fmt.Errorf("RequestSubnet: failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.DebugInfo("Making request to: %s", reqURL)
	log.DebugInfo("Request body: %s", string(jsonBody))

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("CMS : failed to send request: %v", err)
		return nil, fmt.Errorf("RequestSubnet: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("CMS : CMS returned status: %s", resp.Status)
		return nil, fmt.Errorf("CMS server returned non-OK status: %s", resp.Status)
	}

	var addrResp CmsResponse
	if err := json.NewDecoder(resp.Body).Decode(&addrResp); err != nil {
		log.Error("CMS : failed to decode CMS response: %v", err)
		return nil, fmt.Errorf("RequestSubnet: failed to decode response: %w", err)
	}

	return &addrResp, nil
}
