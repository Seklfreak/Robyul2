package helpers

import (
	"sync"

	"bytes"

	"io/ioutil"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/minio/minio-go"
)

var (
	minioBucket string
	minioClient *minio.Client
	minioLock   sync.Mutex
)

// uploads a file to the minio object storage
// objectName	: the name of the file to upload
// data			: the data for the new object
// metadata		: additional metadata attached to the object
func UploadFile(objectName string, data []byte, metadata map[string]string) (err error) {
	// setup minioClient if not yet done
	if minioClient == nil {
		err = setupMinioClient()
		if err != nil {
			return err
		}
	}

	cache.GetLogger().WithField("module", "storage").Infof("uploading " + objectName + " to minio storage")

	options := minio.PutObjectOptions{}

	// add content type
	filetype, err := SniffMime(data)
	if err == nil {
		options.ContentType = filetype
	}

	// add metadata
	if metadata != nil && len(metadata) > 0 {
		options.UserMetadata = metadata
	}

	// upload the data
	_, err = minioClient.PutObject(minioBucket, objectName, bytes.NewReader(data), -1, options)
	return err
}

// TODO: redis cache? file cache?
// retrieves a file from the minio object storage
// objectName	: the name of the file to retrieve
func RetrieveFile(objectName string) (data []byte, err error) {
	// setup minioClient if not yet done
	if minioClient == nil {
		err = setupMinioClient()
		if err != nil {
			return data, err
		}
	}

	cache.GetLogger().WithField("module", "storage").Infof("retrieving " + objectName + " from minio storage")

	// retrieve the object
	minioObject, err := minioClient.GetObject(minioBucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return data, err
	}

	// read the object into a byte slice
	data, err = ioutil.ReadAll(minioObject)
	if err != nil {
		return data, err
	}
	return data, nil
}

// deletes a file from the minio object storage
// objectName	: the file to delete
func DeleteFile(objectName string) (err error) {
	// setup minioClient if not yet done
	if minioClient == nil {
		err = setupMinioClient()
		if err != nil {
			return err
		}
	}

	cache.GetLogger().WithField("module", "storage").Infof("deleting " + objectName + " from minio storage")

	// delete the object
	err = minioClient.RemoveObject(minioBucket, objectName)
	return err
}

// Initialize the minio client object, and creates the bucket if it doesn't exist yet
func setupMinioClient() (err error) {
	minioLock.Lock()
	minioBucket = GetConfig().Path("s3.bucket").Data().(string)
	minioClient, err = minio.New(
		GetConfig().Path("s3.endpoint").Data().(string),
		GetConfig().Path("s3.access_key").Data().(string),
		GetConfig().Path("s3.secret_secret_key").Data().(string),
		true,
	)
	minioLock.Unlock()

	bucketExists, err := minioClient.BucketExists(minioBucket)
	if err != nil {
		return err
	}

	if !bucketExists {
		err = minioClient.MakeBucket(minioBucket, "ams3")
		if err != nil {
			return err
		}
	}

	return err
}
