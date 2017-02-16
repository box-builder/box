package util

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
)

// CheckContext validates that a context, if done, returns the appropriate
// error value stored in it. Otherwise, it will return nil if not terminated.
func CheckContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

// ReadLines reads lines from the file and returns them as []string and any error.
func ReadLines(filename string) ([]string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(string(content)), "\n"), nil
}

// InterfaceListToString converts []interface{} to []string by way of interface{}
func InterfaceListToString(list interface{}) ([]string, error) {
	strList := []string{}

	ifList, ok := list.([]interface{})
	if ok {
		for _, item := range ifList {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New("list is not a list of strings")
			}

			strList = append(strList, str)
		}
	} else {
		return nil, errors.New("object is not a list")
	}

	return strList, nil
}
