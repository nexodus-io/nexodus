package util

import "encoding/json"

func JsonUnmarshal(from map[string]interface{}, to interface{}) error {
	b, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, to)
}

func MustJsonMarshalToString(x interface{}) string {
	bs, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return string(bs)
}
