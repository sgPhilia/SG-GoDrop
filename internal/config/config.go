package config

import "os"

type Config struct {
	Host    string
	Port    int
	TempDir string
}

func Default() *Config {
	return &Config{
		Host:    "0.0.0.0",
		Port:    8080,
		TempDir: tempDirOrDefault(),
	}
}

func tempDirOrDefault() string {
	if dir := os.Getenv("GODROP_TEMP"); dir != "" {
		return dir
	}
	return os.TempDir()
}
