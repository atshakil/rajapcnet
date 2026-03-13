package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddr   string
	DatabasePath string
	StoragePath  string
}

func Load() *Config {
	_ = godotenv.Load(".env.host")

	return &Config{
		ListenAddr:   getEnv("NVR_LISTEN", ":8080"),
		DatabasePath: getEnv("NVR_DB_PATH", "nvr.db"),
		StoragePath:  getEnv("NVR_STORAGE_PATH", "/mnt/nvr/recordings"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
