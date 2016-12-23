package models

type Config struct {
    Id    string `gorethink:"id,omitempty"`
    Guild string `gorethink:"guild"`
    Data  map[string]string `gorethink:"data"`
}