package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddr   string
	DatabasePath string
	StoragePath  string
	JWTSecret    string
	Go2RTCAddr   string // e.g. http://localhost:1984; empty disables WebRTC streaming
}

func Load() *Config {
	_ = godotenv.Load(".env.host")

	return &Config{
		ListenAddr:   getEnv("NVR_LISTEN", ":8080"),
		DatabasePath: getEnv("NVR_DB_PATH", "nvr.db"),
		StoragePath:  getEnv("NVR_STORAGE_PATH", "/mnt/nvr/recordings"),
		JWTSecret:    getEnv("NVR_JWT_SECRET", "change-me-in-production"),
		Go2RTCAddr:   getEnv("NVR_GO2RTC_ADDR", "http://localhost:1984"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
