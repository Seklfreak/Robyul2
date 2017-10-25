package triggers

import (
	"fmt"
	"math/rand"
	"time"
)

type Dog struct{}

func (d *Dog) Triggers() []string {
	return []string{
		"dog",
	}
}

func (d *Dog) Response(trigger string, content string) string {
	randGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	return fmt.Sprintf("WOOF! :dog: \n https://robyul.chat/bundles/robyulweb/images/dog/%d.jpg", randGen.Intn(3)+1)
}
