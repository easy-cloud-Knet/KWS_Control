package service

import (
	"context"
	"fmt"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
)

func GetGuacamoleToken(uuid structure.UUID, ctx *structure.ControlContext) (string, error) {
	core := ctx.FindCoreByVmUUID(uuid)
	if core == nil {
		return "", structure.ErrCoreNotFound(uuid)
	}

	ctx.RLock()
	vm, exists := core.VMInfoIdx[uuid]
	ctx.RUnlock()

	if exists {
		guacClient := client.NewGuacamoleClient(&ctx.Config)

		err := guacClient.Authenticate(context.Background(), string(uuid), vm.GuacPassword)
		if err != nil {
			return "", fmt.Errorf("GetGuacamoleToken: failed to authenticate: %w", err)
		}

		return guacClient.AuthToken(), nil
	} else {
		return "", structure.ErrVmNotFound(uuid)
	}
}
