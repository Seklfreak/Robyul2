package levels

import (
	"math"
	"math/rand"
	"time"
)

func getLevelFromExp(exp int64) int {
	calculatedLevel := 0.1 * math.Sqrt(float64(exp))

	return int(math.Floor(calculatedLevel))
}

func getExpForLevel(level int) int64 {
	if level <= 0 {
		return 0
	}

	calculatedExp := math.Pow(float64(level)/0.1, 2)
	return int64(calculatedExp)
}

func getProgressToNextLevelFromExp(exp int64) int {
	expLevelCurrently := exp - getExpForLevel(getLevelFromExp(exp))
	expLevelNext := getExpForLevel(getLevelFromExp(exp)+1) - getExpForLevel(getLevelFromExp(exp))
	return int(expLevelCurrently / (expLevelNext / 100))
}

func getRandomExpForMessage() int64 {
	min := 10
	max := 15
	rand.Seed(time.Now().Unix())
	return int64(rand.Intn(max-min) + min)
}
