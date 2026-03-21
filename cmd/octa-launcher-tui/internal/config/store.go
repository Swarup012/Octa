package configstore

import (
	"errors"
	"os"
	"path/filepath"

	octaconfig "github.com/Swarup012/solo/pkg/config"
)

const (
	configDirName  = ".octa"
	configFileName = "config.json"
)

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName), nil
}

func Load() (*octaconfig.Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	return octaconfig.LoadConfig(path)
}

func Save(cfg *octaconfig.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	return octaconfig.SaveConfig(path, cfg)
}
