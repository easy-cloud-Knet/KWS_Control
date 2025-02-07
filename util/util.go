package util

import (
	"encoding/json"
	"io"
)

func UnmarshalBodyAndClose[T any](body io.ReadCloser) (T, error) {
	var t T
	err := json.NewDecoder(body).Decode(&t)

	e := body.Close()
	if err == nil {
		err = e
	}

	return t, err
}
