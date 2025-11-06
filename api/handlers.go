package api

import (
	"fmt"
	"net/http"
	"strconv"

	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/redis/go-redis/v9"
)

type handlerContext struct {
	context *vms.ControlContext
	rdb     *redis.Client
}

func Server(portNum int, contextStruct *vms.ControlContext, rdb *redis.Client) error {
	h := handlerContext{
		context: contextStruct,
		rdb:     rdb,
	}

	http.HandleFunc("POST /vm", h.createVm)
	http.HandleFunc("DELETE /vm", h.deleteVm)
	http.HandleFunc("POST /vm/shutdown", h.shutdownVm)
	http.HandleFunc("GET /vm/status", h.vmStatus)
	http.HandleFunc("GET /vm/connect", h.vmConnect)
	http.HandleFunc("POST /vm/redis", h.redis)
	http.HandleFunc("GET /vm/info", h.vmInfo)
	http.HandleFunc("POST /vm/start", h.startVm)

	fmt.Printf("Running server on port %d\n", portNum)
	err := http.ListenAndServe(":"+strconv.Itoa(portNum), nil)
	if err != nil {
		return err
	}

	return nil
}
