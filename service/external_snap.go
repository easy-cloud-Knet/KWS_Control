package service

import (
	"context"
	"fmt"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/client/model"
	vms "github.com/easy-cloud-Knet/KWS_Control/structure"
)

const snapshotPresignTTL = 15 * time.Minute

func ListSnapshots(uuid vms.UUID) ([]string, error) {
	rustfs, err := client.GetRustFSClient()
	if err != nil {
		return nil, fmt.Errorf("ListSnapshots: RustFS unavailable: %w", err)
	}
	keys, err := rustfs.ListObjects(context.Background(), string(uuid), "")
	if err != nil {
		return nil, fmt.Errorf("ListSnapshots %s: %w", uuid, err)
	}
	return keys, nil
}

func DeleteSnapshot(uuid vms.UUID, snapKey string) error {
	rustfs, err := client.GetRustFSClient()
	if err != nil {
		return fmt.Errorf("DeleteSnapshot: RustFS unavailable: %w", err)
	}
	if err := rustfs.DeleteObject(context.Background(), string(uuid), snapKey); err != nil {
		return fmt.Errorf("DeleteSnapshot %s/%s: %w", uuid, snapKey, err)
	}
	return nil
}

// TakeSnapshot generates a presigned PUT URL and sends it to Core to upload the snapshot.
// TODO: wire up Core client call when Core implements the snapshot upload API.
func TakeSnapshot(uuid vms.UUID, snapName string, ctx *vms.ControlContext) error {
	core := ctx.FindCoreByVmUUID(uuid)
	if core == nil {
		return fmt.Errorf("TakeSnapshot: VM %s not found", uuid)
	}

	rustfs, err := client.GetRustFSClient()
	if err != nil {
		return fmt.Errorf("TakeSnapshot: RustFS unavailable: %w", err)
	}

	presignedURL, err := rustfs.PresignPutObject(context.Background(), string(uuid), snapName, snapshotPresignTTL)
	if err != nil {
		return fmt.Errorf("TakeSnapshot %s: failed to generate presigned URL: %w", uuid, err)
	}

	coreClient := client.NewCoreClient(core)
	if _, err := coreClient.TakeExternalSnapshot(context.Background(), model.TakeSnapshotRequest{
		UUID:         uuid,
		SnapKey:      snapName,
		PresignedURL: presignedURL,
	}); err != nil {
		return fmt.Errorf("TakeSnapshot %s: core failed: %w", uuid, err)
	}
	return nil
}
