package helpers

import (
	"net/http"

	"github.com/pkg/errors"
)

func SniffMime(data []byte) (mimetype string, err error) {
	mimetype = http.DetectContentType(data)
	if mimetype == "application/octet-stream" {
		return mimetype, errors.New("unable to sniff filetype")
	}

	return mimetype, nil
}
