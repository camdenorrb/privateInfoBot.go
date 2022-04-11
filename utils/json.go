package utils

import (
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"os"
	"path"
)

func WriteJsonAfterMakeDirs(filePath string, data interface{}) error {

	err := os.MkdirAll(path.Dir(filePath), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to make directories")
	}

	json, err := jsoniter.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal json")
	}

	err = os.WriteFile(filePath, json, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}
