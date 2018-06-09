package idols

type IdolImage struct {
	ImageBytes []byte
	HashString string
	ObjectName string
}

type Idol struct {
	Name         string
	GroupName    string
	Gender       string
	NameAndGroup string
	Images       []IdolImage
}
