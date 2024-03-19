package util

import (
	"encoding/json"
	"fmt"
)

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

type jsonStringer struct {
	value interface{}
}

func (s jsonStringer) String() string {
	bs, err := json.Marshal(s.value)
	if err != nil {
		return "json marshal failed: " + err.Error()
	}
	return string(bs)
}

func JsonStringer(x interface{}) fmt.Stringer {
	return jsonStringer{value: x}
}
