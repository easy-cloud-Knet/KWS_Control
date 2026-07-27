package model

import "github.com/easy-cloud-Knet/KWS_Control/structure"

// TakeSnapshotRequest asks Core to take an external snapshot and upload to RustFS via presigned PUT URL.
type TakeSnapshotRequest struct {
	UUID         structure.UUID `json:"uuid"`
	SnapKey      string         `json:"snapKey"`
	PresignedURL string         `json:"presignedUrl,omitempty"`
}

type TakeSnapshotResponse struct {
	UUID    string `json:"uuid"`
	SnapKey string `json:"snapKey"`
}
