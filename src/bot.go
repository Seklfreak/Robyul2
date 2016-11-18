package main

import (
	"io/ioutil"
	"./karen"
)

func main() {
	token, _ := ioutil.ReadFile("token")
	karen.Initialize(token)
}