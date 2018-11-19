package notifications

import "github.com/Seklfreak/Robyul2/models"

type entryWithBytes struct {
	*models.NotificationsEntry
	KeywordBytes []byte
}

type delimiterCombination struct {
	Start []byte
	End   []byte
}
