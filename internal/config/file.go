package config

import (
	"fmt"
	"os"
)

func readConfig(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", path)
	}

	return data, err
}

func Load(path string) (*Config, error) {
	buf, err := readConfig(path)
	if err != nil {
		return nil, err
	}

	return Parse(buf)
}
