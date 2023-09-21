package util

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func Getenv(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetenvDuration(name, defaultValue string) (time.Duration, error) {
	valueStr := Getenv(name, defaultValue)
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return value, fmt.Errorf("invalid value for %s (%s): %w", name, valueStr, err)
	}
	return value, nil
}

func GetenvInt(name, defaultValue string) (int, error) {
	valueStr := Getenv(name, defaultValue)
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return value, fmt.Errorf("invalid value for %s (%s): %w", name, valueStr, err)
	}
	return value, nil
}
