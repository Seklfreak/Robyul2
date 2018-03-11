package helpers

import (
	"sync"

	"bytes"

	"io/ioutil"

	"os"

	"path/filepath"

	"errors"

	"fmt"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
	"github.com/kennygrant/sanitize"
	"github.com/minio/minio-go"
)

var (
	minioBucket string
	minioClient *minio.Client
	minioLock   sync.Mutex
)

// TODO: watch cache folder size

// retrieves a file by md5 hash
// currently supported file sources: custom commands
// hash	: the md5 hash
func RetrieveFileByHash(hash string) (filename, filetype string, data []byte, err error) {
	var entryBucket models.CustomCommandsEntry
	err = MdbOne(
		MdbCollection(models.CustomCommandsTable).Find(bson.M{"storagehash": hash}),
		&entryBucket,
	)
	if err == nil {
		data, err = RetrieveFile(entryBucket.StorageObjectName)
		RelaxLog(err)
		if err != nil {
			return "", "", nil, err
		}
		return entryBucket.StorageFilename, entryBucket.StorageMimeType, data, nil
	}
	return "", "", nil, errors.New("file not found")
}

// Gets a public link for a file
// filename		: the name of the file
// filetype		: the type of the file
// objectName	: the minio object name of the file
func GetPublicFileLink(filename, filehash string) (link string) {
	dots := strings.Count(filename, ".")
	if dots > 1 {
		filename = strings.Replace(filename, ".", "-", dots-1)
	}
	//filename = strings.Replace("_", "-", -1)
	return fmt.Sprintf(GetConfig().Path("imageproxy.base_url").Data().(string),
		filehash, filename)
}

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
	_, err = minioClient.PutObject(minioBucket, sanitize.BaseName(objectName), bytes.NewReader(data), -1, options)
	return err
}

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

	data = getBucketCache(objectName)
	if data != nil {
		cache.GetLogger().WithField("module", "storage").Infof("retrieving " + objectName + " from minio cache")
		return data, nil
	}

	cache.GetLogger().WithField("module", "storage").Infof("retrieving " + objectName + " from minio storage")

	// retrieve the object
	minioObject, err := minioClient.GetObject(minioBucket, sanitize.BaseName(objectName), minio.GetObjectOptions{})
	if err != nil {
		return data, err
	}

	// read the object into a byte slice
	data, err = ioutil.ReadAll(minioObject)
	if err != nil {
		return data, err
	}

	go func() {
		defer Recover()
		cache.GetLogger().WithField("module", "storage").Infof("caching " + objectName + " into minio cache")
		err = setBucketCache(objectName, data)
		RelaxLog(err)
	}()

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

	go func() {
		defer Recover()
		cache.GetLogger().WithField("module", "storage").Infof("deleting " + objectName + " from minio cache")
		err = deleteBucketCache(objectName)
		RelaxLog(err)
	}()

	// delete the object
	err = minioClient.RemoveObject(minioBucket, sanitize.BaseName(objectName))
	return err
}

func getBucketCache(objectName string) (data []byte) {
	var err error

	if _, err = os.Stat(getObjectPath(objectName)); os.IsNotExist(err) {
		return nil
	}

	data, err = ioutil.ReadFile(getObjectPath(objectName))
	if err != nil {
		return nil
	}

	return data
}

func setBucketCache(objectName string, data []byte) (err error) {
	if _, err = os.Stat(filepath.Dir(getObjectPath(objectName))); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(getObjectPath(objectName)), os.ModePerm)
		if err != nil {
			return err
		}
	}

	err = ioutil.WriteFile(getObjectPath(objectName), data, 0644)
	return err
}

func deleteBucketCache(objectName string) (err error) {
	if _, err = os.Stat(getObjectPath(objectName)); os.IsNotExist(err) {
		return nil
	}

	err = os.Remove(getObjectPath(objectName))
	return err
}

func getObjectPath(objectName string) (path string) {
	return GetConfig().Path("cache_folder").Data().(string) + "/minio-" + GetConfig().Path("s3.bucket").Data().(string) + "/" + sanitize.BaseName(objectName)
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
