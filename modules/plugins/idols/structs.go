package idols

type IdolImage struct {
	ImageBytes []byte
	HashString string
	ObjectName string
}

type Idol struct {
	BiasName     string
	GroupName    string
	Gender       string
	NameAndGroup string
	BiasImages   []IdolImage
}
