package pages

import (
	"encoding/json"
	"io/ioutil"
)

func readAndUnmarshal(filePath string, v interface{}) error {
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, v)
	file = []byte("")
	return err
}
