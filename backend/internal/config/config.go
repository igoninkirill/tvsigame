package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DatabaseURL   string
	MediaDir      string
	StaticDir     string
	MaxUploadMB   int64
	RoomTTLHours  int
	WSPingPeriod  int // seconds
	HostGraceSec  int // grace period before host is considered disconnected
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func Load() (Config, error) {
	c := Config{
		Port:         getint("APP_PORT", 8080),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		MediaDir:     getenv("MEDIA_DIR", "./data/media"),
		StaticDir:    os.Getenv("STATIC_DIR"),
		MaxUploadMB:  int64(getint("MAX_UPLOAD_MB", 512)),
		RoomTTLHours: getint("ROOM_TTL_HOURS", 72),
		WSPingPeriod: getint("WS_PING_PERIOD", 5),
		HostGraceSec: getint("HOST_GRACE_SEC", 15),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}
