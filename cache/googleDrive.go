package cache

import (
	"errors"
	"sync"

	"google.golang.org/api/drive/v3"
)

var (
	googleDriveService *drive.Service
	googleDriveMutex   sync.RWMutex
)

func SetGoogleDriveService(service *drive.Service) {
	googleDriveMutex.Lock()
	defer googleDriveMutex.Unlock()

	googleDriveService = service
}

func GetGoogleDriveService() *drive.Service {
	googleDriveMutex.RLock()
	defer googleDriveMutex.RUnlock()

	if googleDriveService == nil {
		panic(errors.New("Tried to use google drive service before cache#SetGoogleDriveService() was called"))
	}

	return googleDriveService
}

// HasGoogleDrive simple check to confirm drive session was set
func HasGoogleDrive() bool {
	return !(googleDriveService == nil)
}
