package models

type Config struct {
    Guild string `gorethink:"guild"`
    Data  map[string]string `gorethink:"data"`
}