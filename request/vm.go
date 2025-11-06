package request

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/request/model"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

type CoreClient struct {
	baseURL string
	client  *http.Client
}

func NewCoreClient(core *structure.Core) *CoreClient {
	return &CoreClient{
		baseURL: fmt.Sprintf("http://%s:%d", core.IP, core.Port),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *CoreClient) doRequest(context context.Context, method, path string, requestBody interface{}, responseBody interface{}) error {
	log := util.GetLogger()

	var reqBodyReader io.Reader
	if requestBody != nil {
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}

		log.Println("(-> Core) request body:", string(jsonData))
		reqBodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(context, method, c.baseURL+path, reqBodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	if responseBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
			return fmt.Errorf("failed to decode response body: %w", err)
		}
	}

	return nil
}

func (c *CoreClient) CreateVM(context context.Context, req model.CreateVMRequest) (model.CreateVMResponse, error) {
	var response model.CreateVMResponse
	err := c.doRequest(context, http.MethodPost, "/createVM", req, &response)
	if err != nil {
		return model.CreateVMResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) DeleteVM(context context.Context, req model.DeleteVMRequest) (model.DeleteVMResponse, error) {
	var response model.DeleteVMResponse
	err := c.doRequest(context, http.MethodPost, "/DeleteVM", req, &response)
	if err != nil {
		return model.DeleteVMResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) GetCoreMachineCpuInfo(context context.Context) (*model.CoreMachineCpuInfoResponse, error) {
	var response model.CoreResponse[model.CoreMachineCpuInfoResponse]
	err := c.doRequest(context, http.MethodGet, "/getStatusHost", model.GetMachineStatusRequest{
		HostDataType: model.CpuInfo,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.Information, nil
}

func (c *CoreClient) GetCoreMachineDiskInfo(context context.Context) (*model.CoreMachineDiskInfoResponse, error) {
	var response model.CoreResponse[model.CoreMachineDiskInfoResponse]
	err := c.doRequest(context, http.MethodGet, "/getStatusHost", model.GetMachineStatusRequest{
		HostDataType: model.DiskInfoHi,
	}, &response)
	if err != nil {
		return nil, err
	}

	return response.Information, nil
}

func (c *CoreClient) GetCoreMachineMemoryInfo(context context.Context) (*model.CoreMachineMemoryInfoResponse, error) {
	var response model.CoreResponse[model.CoreMachineMemoryInfoResponse]
	err := c.doRequest(context, http.MethodGet, "/getStatusHost", model.GetMachineStatusRequest{
		HostDataType: model.MemInfo,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.Information, nil
}

func (c *CoreClient) GetVMCpuInfo(ctx context.Context, uuid structure.UUID) (model.CoreMachineCpuInfoResponse, error) {
	var response model.CoreMachineCpuInfoResponse
	req := model.GetVMStatusRequest{
		UUID:     uuid,
		DataType: model.CpuInfo,
	}
	err := c.doRequest(ctx, http.MethodGet, "/getStatusUUID", req, &response)
	if err != nil {
		return model.CoreMachineCpuInfoResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) GetVMDiskInfo(ctx context.Context, uuid structure.UUID) (model.CoreMachineDiskInfoResponse, error) {
	var response model.CoreMachineDiskInfoResponse
	req := model.GetVMStatusRequest{
		UUID:     uuid,
		DataType: model.DiskInfoHi,
	}
	err := c.doRequest(ctx, http.MethodGet, "/getStatusUUID", req, &response)
	if err != nil {
		return model.CoreMachineDiskInfoResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) GetVMMemoryInfo(ctx context.Context, uuid structure.UUID) (model.CoreMachineMemoryInfoResponse, error) {
	var response model.CoreMachineMemoryInfoResponse
	req := model.GetVMStatusRequest{
		UUID:     uuid,
		DataType: model.MemInfo,
	}
	err := c.doRequest(ctx, http.MethodGet, "/getStatusUUID", req, &response)
	if err != nil {
		return model.CoreMachineMemoryInfoResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) StartVM(ctx context.Context, req model.StartVMRequest) (model.StartVMResponse, error) {
	var response model.StartVMResponse
	err := c.doRequest(ctx, http.MethodPost, "/BOOTVM", req, &response)
	if err != nil {
		return model.StartVMResponse{}, err
	}
	return response, nil
}

func (c *CoreClient) ForceShutdownVM(ctx context.Context, req model.ForceShutdownVMRequest) (model.ForceShutdownVMResponse, error) {
	var response model.ForceShutdownVMResponse
	err := c.doRequest(ctx, http.MethodPost, "/forceShutDownUUID", req, &response)
	if err != nil {
		return model.ForceShutdownVMResponse{}, err
	}
	return response, nil
}
