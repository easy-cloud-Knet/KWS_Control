package WorkerCont

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	vms "github.com/easy-cloud-Knet/KWS_Control/vm"
)

type CoreRequestTask[P any, R any] struct {
	Core     *vms.Core
	Endpoint string
	Request  P
	Response R
}

func (t *CoreRequestTask[P, R]) Await() (body R, err error) {
	jsonData, err := json.Marshal(t.Request)
	if err != nil {
		return
	}
	//fmt.Println(t.Request)
	requestUrl := url.URL{
		Scheme: "http",
		Host:   t.Core.IP + ":" + strconv.Itoa(t.Core.Port),
		Path:   t.Endpoint,
	}
	fmt.Println(requestUrl.String())
	resp, err := http.Post(requestUrl.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(Body io.ReadCloser) {
		e := Body.Close()
		if err == nil {
			err = e
		}
	}(resp.Body)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	fmt.Println(resp.Body)
	err = json.Unmarshal(b, &body)
	return
}

func NewCreateVMTask(core *vms.Core, param CreateVMParam) CoreRequestTask[CreateVMParam, string] {
	return CoreRequestTask[CreateVMParam, string]{
		Core:     core,
		Endpoint: "/createVM",
		Request:  param,
	}
}

func NewDeleteVMTask(core *vms.Core, param CreateVMParam) CoreRequestTask[CreateVMParam, string] {
	return CoreRequestTask[CreateVMParam, string]{
		Core:     core,
		Endpoint: "/deleteVM",
		Request:  param,
	}
}
