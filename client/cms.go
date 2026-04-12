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

// fmt.Sprintf("%s/New/Instance", CMS_HOST)
func NewCmsClient() *CmsClient {
	log := util.GetLogger()
	CMS_HOST := os.Getenv("CMS_HOST")
	if CMS_HOST == "" {
		CMS_HOST = "localhost:8080"
		log.Warn("CMS_HOST env not set, defaulting to %s", CMS_HOST, true)
	}
	log.Println("NewCmsClient (-> CMS) baseURL:", CMS_HOST)
	return &CmsClient{
		baseURL: CMS_HOST,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *CmsClient) CmsRequest(Subnet string) (*CmsResponse, error) {
	req_url := fmt.Sprintf("http://%s/New/Instance", c.baseURL)
	reqBody := CmsRequest{Subnet: Subnet}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CMS request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, req_url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create CMS request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send CMS request to %s: %w", req_url, err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CMS returned non-OK status: %s", resp.Status)
	}

	var addrResp CmsResponse
	if err := json.NewDecoder(resp.Body).Decode(&addrResp); err != nil {
		return nil, fmt.Errorf("failed to decode CMS response: %w", err)
	}

	return &addrResp, nil
}
