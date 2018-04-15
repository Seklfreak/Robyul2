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

	"time"

	"strconv"

	"encoding/binary"

	"mime"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
	"github.com/kennygrant/sanitize"
	"github.com/minio/minio-go"
	uuid "github.com/satori/go.uuid"
)

var (
	minioBucket string
	minioClient *minio.Client
	minioLock   sync.Mutex
)

// TODO: watch cache folder size

type AddFileMetadata struct {
	Filename           string            // the actual file name, can be empty
	ChannelID          string            // the source channel ID, can be empty, but should be set if possible
	UserID             string            // the source user ID, can be empty, but should be set if possible
	GuildID            string            // the source guild ID, can be empty but should be set if possible, will be set automatically if ChannelID has been set
	AdditionalMetadata map[string]string // additional metadata attached to the object
}

// TODO: prevent duplicates
// Stores a file
// name		: the name of the new object, can be empty to generate an unique name
// data		: the file data
// metadata	: metadata attached to the object
// source	: the source name for the file, for example the module name, can not be empty
// public	: if true file will be available via the website proxy
func AddFile(name string, data []byte, metadata AddFileMetadata, source string, public bool) (objectName string, err error) {
	// check if source is set
	if source == "" {
		return "", errors.New("source can not be empty")
	}
	// check if user uploads are disabled for given userID, if userID is given
	if metadata.UserID != "" && UseruploadsIsDisabled(metadata.UserID) {
		return "", errors.New("uploads are disabled for this user")
	}
	// set new object name
	objectName = name
	if objectName == "" {
		// generate unique filename
		newID, err := uuid.NewV4()
		if err != nil {
			return "", err
		}
		objectName = newID.String()
	}
	// retrieve guildID if channelID is set
	guildID := metadata.GuildID
	if metadata.ChannelID != "" {
		channel, err := GetChannel(metadata.ChannelID)
		RelaxLog(err)
		if err == nil {
			guildID = channel.GuildID
		}
	}
	// get filetype
	filetype, _ := SniffMime(data)
	// get filesize
	filesize := binary.Size(data)
	// update metadata
	if metadata.AdditionalMetadata == nil {
		metadata.AdditionalMetadata = make(map[string]string, 0)
	}
	metadata.AdditionalMetadata["filename"] = metadata.Filename
	metadata.AdditionalMetadata["userid"] = metadata.UserID
	metadata.AdditionalMetadata["guildid"] = guildID
	metadata.AdditionalMetadata["channelid"] = metadata.ChannelID
	metadata.AdditionalMetadata["source"] = source
	metadata.AdditionalMetadata["mimetype"] = filetype
	metadata.AdditionalMetadata["filesize"] = strconv.Itoa(filesize)
	metadata.AdditionalMetadata["public"] = "no"
	if public {
		metadata.AdditionalMetadata["public"] = "yes"
	}
	// upload file
	err = uploadFile(objectName, data, metadata.AdditionalMetadata)
	if err != nil {
		return "", err
	}
	// store in database
	err = MDbUpsert(
		models.StorageTable,
		bson.M{"objectname": objectName},
		models.StorageEntry{
			ObjectName:     objectName,
			ObjectNameHash: GetMD5Hash(objectName),
			UploadDate:     time.Now(),
			Filename:       metadata.Filename,
			UserID:         metadata.UserID,
			GuildID:        guildID,
			ChannelID:      metadata.ChannelID,
			Source:         source,
			MimeType:       filetype,
			Filesize:       filesize,
			Public:         public,
			Metadata:       metadata.AdditionalMetadata,
		},
	)
	if err != nil {
		return "", err
	}
	// warm up cache for public files
	if public {
		go func() {
			defer Recover()
			link, err := GetFileLink(objectName)
			Relax(err)
			_, err = NetGetUAWithError(link, DEFAULT_UA)
			if err != nil {
				cache.GetLogger().WithField("module", "storage").Warnf(
					"error warming up #%s: %s", objectName, err.Error(),
				)
			}
		}()
	}
	// return new objectName
	return objectName, nil
}

// retrieves information about a file
// objectName	: the name of the file to retrieve
func RetrieveFileInformation(objectName string) (info models.StorageEntry, err error) {
	err = MdbOneWithoutLogging(
		MdbCollection(models.StorageTable).Find(bson.M{"objectname": objectName}),
		&info,
	)
	return info, err
}

// retrieves a file
// objectName	: the name of the file to retrieve
func RetrieveFile(objectName string) (data []byte, err error) {
	// setup minioClient if not yet done
	if minioClient == nil {
		err = setupMinioClient()
		if err != nil {
			return data, err
		}
	}

	// Increase MongoDB RetrievedCount
	go func() {
		defer Recover()
		err := MDbUpdateQueryWithoutLogging(models.StorageTable, bson.M{"objectname": objectName}, bson.M{"$inc": bson.M{"retrievedcount": 1}})
		if err != nil && !IsMdbNotFound(err) {
			RelaxLog(err)
		}
	}()

	data = getBucketCache(objectName)
	if data != nil {
		cache.GetLogger().WithField("module", "storage").Infof("retrieving " + objectName + " from minio cache")
		return data, nil
	}

	cache.GetLogger().WithField("module", "storage").Infof("retrieving " + objectName + " from minio storage")

	// retrieve the object
	minioObject, err := minioClient.GetObject(minioBucket, sanitize.BaseName(objectName), minio.GetObjectOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "Please reduce your request rate.") {
			cache.GetLogger().WithField("module", "storage").Infof("object storage ratelimited, waiting for one second, then retrying")
			time.Sleep(1 * time.Second)
			return RetrieveFile(objectName)
		}
		if strings.Contains(err.Error(), "net/http") || strings.Contains(err.Error(), "timeout") {
			cache.GetLogger().WithField("module", "storage").Infof("network error retrieving, waiting for one second, then retrying")
			time.Sleep(1 * time.Second)
			return RetrieveFile(objectName)
		}
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
		err := setBucketCache(objectName, data)
		RelaxLog(err)
	}()

	return data, nil
}

// retrieves a file without logging
// objectName	: the name of the file to retrieve
func RetrieveFileWithoutLogging(objectName string) (data []byte, err error) {
	// setup minioClient if not yet done
	if minioClient == nil {
		err = setupMinioClient()
		if err != nil {
			return data, err
		}
	}

	// Increase MongoDB RetrievedCount
	go func() {
		defer Recover()
		err := MDbUpdateQueryWithoutLogging(models.StorageTable, bson.M{"objectname": objectName}, bson.M{"$inc": bson.M{"retrievedcount": 1}})
		if err != nil && !IsMdbNotFound(err) {
			RelaxLog(err)
		}
	}()

	data = getBucketCache(objectName)
	if data != nil {
		return data, nil
	}

	// retrieve the object
	minioObject, err := minioClient.GetObject(minioBucket, sanitize.BaseName(objectName), minio.GetObjectOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "Please reduce your request rate.") {
			cache.GetLogger().WithField("module", "storage").Infof("object storage ratelimited, waiting for one second, then retrying")
			time.Sleep(1 * time.Second)
			return RetrieveFileWithoutLogging(objectName)
		}
		if strings.Contains(err.Error(), "net/http") || strings.Contains(err.Error(), "timeout") {
			cache.GetLogger().WithField("module", "storage").Infof("network error retrieving, waiting for one second, then retrying")
			time.Sleep(1 * time.Second)
			return RetrieveFileWithoutLogging(objectName)
		}
		return data, err
	}

	// read the object into a byte slice
	data, err = ioutil.ReadAll(minioObject)
	if err != nil {
		return data, err
	}

	go func() {
		defer Recover()
		err := setBucketCache(objectName, data)
		RelaxLog(err)
	}()

	return data, nil
}

// Retrieves a file by the object name md5 hash
// currently supported file sources: custom commands
// hash	: the md5 hash
func RetrieveFileByHash(hash string) (filename, filetype string, data []byte, err error) {
	var entryBucket models.StorageEntry
	err = MdbOneWithoutLogging(
		MdbCollection(models.StorageTable).Find(bson.M{"objectnamehash": hash}),
		&entryBucket,
	)
	if err != nil {
		// try fallback to deprecated custom commands object storage
		var entryBucket models.CustomCommandsEntry
		err = MdbOneWithoutLogging(
			MdbCollection(models.CustomCommandsTable).Find(bson.M{"storagehash": hash}),
			&entryBucket,
		)
		if err != nil {
			return "", "", nil, errors.New("file not found")
		}
		data, err = RetrieveFile(entryBucket.StorageObjectName)
		RelaxLog(err)
		if err != nil {
			return "", "", nil, err
		}
		return entryBucket.StorageFilename, entryBucket.StorageMimeType, data, nil
	}

	data, err = RetrieveFile(entryBucket.ObjectName)
	RelaxLog(err)
	if err != nil {
		return "", "", nil, err
	}
	return entryBucket.Filename, entryBucket.MimeType, data, nil
}

// Retrieves files by additional object metadta
// currently supported file sources: custom commands
// hash	: the md5 hash
func RetrieveFilesByAdditionalObjectMetadata(key, value string) (objectNames []string, err error) {
	var entryBucket []models.StorageEntry
	err = MDbIter(MdbCollection(models.StorageTable).Find(
		bson.M{"metadata." + strings.ToLower(key): value},
	)).All(&entryBucket)

	objectNames = make([]string, 0)
	if entryBucket != nil && len(entryBucket) > 0 {
		for _, entry := range entryBucket {
			objectNames = append(objectNames, entry.ObjectName)
		}
	}

	if len(objectNames) < 1 {
		return nil, errors.New("none matching files found")
	}

	return objectNames, nil
}

// Retrieves a file's public url, returns an error if file is not public
// objectName	:  the name of the object
func GetFileLink(objectName string) (url string, err error) {
	var entryBucket models.StorageEntry
	err = MdbOneWithoutLogging(
		MdbCollection(models.StorageTable).Find(bson.M{"objectname": objectName}),
		&entryBucket,
	)
	if err != nil {
		return "", err
	}

	if !entryBucket.Public {
		return "", errors.New("this file is not available publicly")
	}

	filename := entryBucket.Filename
	if filename == "" {
		filename = "robyul"
		extensions, err := mime.ExtensionsByType(entryBucket.MimeType)
		if err == nil && extensions != nil && len(extensions) >= 0 {
			filename += extensions[0]
		}
	}

	url = GeneratePublicFileLink(filename, entryBucket.ObjectNameHash)

	return url, nil
}

// Deletes a file
// objectName	: the name of the object
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

	// delete mongo db entry
	go func() {
		defer Recover()
		err := MdbDeleteQuery(models.StorageTable, bson.M{"objectname": objectName})
		if err != nil && !IsMdbNotFound(err) {
			RelaxLog(err)
		}
	}()

	return err
}

// Gets a public link for a file
// filename		: the name of the file
// filetype		: the type of the file
func GeneratePublicFileLink(filename, filehash string) (link string) {
	dots := strings.Count(filename, ".")
	if dots > 1 {
		filename = strings.Replace(filename, ".", "-", dots-1)
	}
	return fmt.Sprintf(GetConfig().Path("imageproxy.base_url").Data().(string),
		filehash, filename)
}

// TODO: prevent overwrites
// uploads a file to the minio object storage
// objectName	: the name of the file to upload
// data			: the data for the new object
// metadata		: additional metadata attached to the object
func uploadFile(objectName string, data []byte, metadata map[string]string) (err error) {
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
