package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConflictsError(t *testing.T) {
	e := NewConflictsError("03c47bd2-170c-4b19-bdc0-d7e18d190dcf")
	b, err := json.Marshal(e)
	require.NoError(t, err)
	require.Equal(t, `{"id":"03c47bd2-170c-4b19-bdc0-d7e18d190dcf","error":"resource already exists"}`, string(b))

	var e2 ConflictsError
	err = json.Unmarshal(b, &e2)
	require.NoError(t, err)
	require.Equal(t, e, e2)
}
