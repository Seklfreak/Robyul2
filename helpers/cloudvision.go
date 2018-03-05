package helpers

import (
	"context"
	"os"

	"io"

	vision "cloud.google.com/go/vision/apiv1"
	vision2 "google.golang.org/genproto/googleapis/cloud/vision/v1"
)

var visionClient *vision.ImageAnnotatorClient

func PictureIsSafe(reader io.Reader) (safe bool) {
	var err error
	ctx := context.Background()

	if visionClient == nil {
		os.Setenv(
			"GOOGLE_APPLICATION_CREDENTIALS",
			GetConfig().Path("google.client_credentials_json_location").Data().(string),
		)

		visionClient, err = vision.NewImageAnnotatorClient(ctx)
		if err != nil {
			RelaxLog(err)
			return false
		}
	}

	image, err := vision.NewImageFromReader(reader)
	if err != nil {
		RelaxLog(err)
		return false
	}

	safeData, err := visionClient.DetectSafeSearch(ctx, image, nil)
	if err != nil {
		RelaxLog(err)
		return false
	}

	if safeData.GetAdult() == vision2.Likelihood_VERY_LIKELY {
		return false
	}

	if safeData.GetMedical() == vision2.Likelihood_VERY_LIKELY {
		return false
	}

	if safeData.GetViolence() == vision2.Likelihood_VERY_LIKELY {
		return false
	}

	if safeData.GetRacy() == vision2.Likelihood_VERY_LIKELY {
		return false
	}

	return true
}
