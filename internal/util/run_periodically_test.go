package util_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestRunPeriodically(t *testing.T) {
	// Crudely check that this runs a bunch of times when running every millisecond
	// within 100 milliseconds
	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	util.RunPeriodically(ctx, time.Nanosecond, func() {
		count++
	})
	assert.Greater(t, count, 2)

	// Make sure this only runs once
	count = 0
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*150)
	defer cancel()
	util.RunPeriodically(ctx, time.Millisecond*100, func() { count++ })
	assert.Equal(t, count, 1)
}

func TestCheckPeriodically(t *testing.T) {
	// Make sure a true result stops the check
	cond, err := util.CheckPeriodically(context.Background(), time.Millisecond, func() (bool, error) {
		return true, nil
	})
	assert.True(t, cond)
	assert.Nil(t, err)

	// Keep checking if the condition is false
	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	cond, err = util.CheckPeriodically(ctx, time.Millisecond*10, func() (bool, error) {
		count++
		return false, nil
	})
	assert.False(t, cond)
	assert.Nil(t, err)
	assert.Greater(t, count, 5)

	// Stop checking on error
	cond, err = util.CheckPeriodically(context.Background(), time.Millisecond, func() (bool, error) {
		return false, errors.New("test error")
	})
	assert.False(t, cond)
	assert.NotNil(t, err)
}
