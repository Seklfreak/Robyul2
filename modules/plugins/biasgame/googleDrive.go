package biasgame

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// This file contains old functions used when biasgame worked with google drive.
// This is no longer the case, however these functions can still be used to migrate files from a google drive to the biasgame if needed.

const (
	DRIVE_SEARCH_TEXT = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
)

// runGoogleDriveMigration Should only be run on rare occasions when issues occur with object storage or setting up a new object storage
//  note: takes a very long time to complete
func runGoogleDriveMigration(msg *discordgo.Message) {
	girlFolderId := helpers.GetConfig().Path("biasgame.girl_folder_id").Data().(string)
	boyFolderId := helpers.GetConfig().Path("biasgame.boy_folder_id").Data().(string)

	// get files from drive
	girlFiles := getFilesFromDriveFolder(girlFolderId)
	boyFiles := getFilesFromDriveFolder(boyFolderId)
	allFiles := append(girlFiles, boyFiles...)

	amountMigrated := 0

	// confirm files were found
	if len(allFiles) > 0 {

		bgLog().Info("--Migrating google drive biasgame images to object storage. Total images found: ", len(allFiles))
		for _, file := range allFiles {
			// determine gender from folder
			var gender string
			if file.Parents[0] == girlFolderId {
				gender = "girl"
			} else {
				gender = "boy"
			}

			// get bias name and group name from file name
			groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

			biasEntry := models.BiasGameIdolEntry{
				ID:        "",
				DriveID:   file.Id,
				Gender:    gender,
				GroupName: strings.Split(groupBias, "_")[0],
				Name:      strings.Split(groupBias, "_")[1],
			}

			// check if a record with this drive id already exists
			//  this means its been migrated before and should not be remigrated
			count, err := helpers.MdbCount(models.BiasGameIdolsTable, bson.M{"driveid": biasEntry.DriveID})
			if err != nil {
				bgLog().Errorf("Error getting count for drive id '%s'. Error: %s", biasEntry.DriveID, err.Error())
				continue
			}
			if count != 0 {
				bgLog().Infof("Drive id '%s' has already been migrated. Skipping", biasEntry.DriveID)
				continue
			}
			bgLog().Infof("Migrating Drive id '%s'. Idol Name: %s | Group Name: %s", biasEntry.DriveID, biasEntry.Name, biasEntry.GroupName)

			// get image
			res, err := pester.Get(file.WebContentLink)
			helpers.Relax(err)
			imgBytes, err := ioutil.ReadAll(res.Body)

			// store file in object storage
			objectName, err := helpers.AddFile("", imgBytes, helpers.AddFileMetadata{
				Filename:           file.WebContentLink,
				ChannelID:          msg.ChannelID,
				UserID:             msg.Author.ID,
				AdditionalMetadata: nil,
			}, "biasgame", false)

			// set object name
			biasEntry.ObjectName = objectName

			// insert file to mongodb
			_, err = helpers.MDbInsert(models.BiasGameIdolsTable, biasEntry)
			if err != nil {
				bgLog().Errorf("Error migrating drive id '%s'. Error: %s", biasEntry.DriveID, err.Error())
			}
			amountMigrated++
		}
		bgLog().Info("--Google drive migration complete--")
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Migration Complete. Files Migrated: %d", amountMigrated))

	} else {
		bgLog().Warn("No biasgame file found!")
	}
}

// getFilesFromDriveFolder
func getFilesFromDriveFolder(folderId string) []*drive.File {
	driveService := cache.GetGoogleDriveService()

	// get girls image from google drive
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).Do()
	if err != nil {
		return nil
	}
	allFiles := results.Files

	// retry for more bias images if needed
	pageToken := results.NextPageToken
	for pageToken != "" {
		results, err = driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).PageToken(pageToken).Do()
		pageToken = results.NextPageToken
		if len(results.Files) > 0 {
			allFiles = append(allFiles, results.Files...)
		} else {
			break
		}
	}

	return allFiles
}
