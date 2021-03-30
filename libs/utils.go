package libs

import (
	"encoding/json"
	"io"
	"io/ioutil"
)

const TimeFormat = "2006-01-02 15:04:05"

func DecodeJSON(r io.Reader, v interface{}) error {
	defer func() {
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			panic(err)
		}

	}()
	return json.NewDecoder(r).Decode(v)
}
