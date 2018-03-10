package helpers

import (
	"net/http"

	"github.com/pkg/errors"
)

func SniffMime(data []byte) (mimetype string, err error) {
	if len(data) < 512 {
		return mimetype, errors.New("file too small to sniff mime type")
	}

	// sniff filetype from first 512 bytes
	contentType := http.DetectContentType(data[0:511])

	return contentType, nil
}
