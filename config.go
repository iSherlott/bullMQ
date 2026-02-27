package bullmq

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr      string
	Host      string
	Port      int
	Password  string
	DB        int
	UseTLS    bool
	KeyPrefix string
}

func (config Config) normalize() Config {
	addr := strings.TrimSpace(config.Addr)
	host := strings.TrimSpace(config.Host)
	port := config.Port
	if port <= 0 {
		if config.UseTLS {
			port = 6380
		} else {
			port = 6379
		}
	}

	if addr == "" {
		addr = buildAddr(host, port)
	}

	if host == "" {
		host = extractHost(addr)
	}

	keyPrefix := config.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = strings.TrimSpace(os.Getenv("BULLMQ_KEY_PREFIX"))
	}
	if keyPrefix == "" {
		keyPrefix = "bull"
	}

	return Config{
		Addr:      addr,
		Host:      host,
		Port:      port,
		Password:  config.Password,
		DB:        config.DB,
		UseTLS:    config.UseTLS,
		KeyPrefix: keyPrefix,
	}
}

func LoadConfigFromEnv(envFiles ...string) (Config, error) {
	if len(envFiles) > 0 {
		if err := godotenv.Load(envFiles...); err != nil {
			return Config{}, err
		}
	} else {
		_ = godotenv.Load()
	}

	database := 0
	databaseFromEnv := strings.TrimSpace(os.Getenv("REDIS_DB"))
	if databaseFromEnv != "" {
		parsed, err := strconv.Atoi(databaseFromEnv)
		if err != nil {
			return Config{}, err
		}
		database = parsed
	}

	port := 0
	portFromEnv := strings.TrimSpace(os.Getenv("REDIS_PORT"))
	if portFromEnv != "" {
		parsed, err := strconv.Atoi(portFromEnv)
		if err != nil {
			return Config{}, err
		}
		port = parsed
	}

	useTLS := false
	sslFromEnv := strings.ToLower(strings.TrimSpace(os.Getenv("REDIS_SSL")))
	if sslFromEnv == "1" || sslFromEnv == "true" || sslFromEnv == "yes" {
		useTLS = true
	}

	keyPrefix := strings.TrimSpace(os.Getenv("BULLMQ_KEY_PREFIX"))
	if keyPrefix == "" {
		keyPrefix = "bull"
	}

	config := Config{
		Addr:      strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		Host:      strings.TrimSpace(os.Getenv("REDIS_HOST")),
		Port:      port,
		Password:  strings.TrimSpace(os.Getenv("REDIS_PASSWORD")),
		DB:        database,
		UseTLS:    useTLS,
		KeyPrefix: keyPrefix,
	}.normalize()

	if strings.TrimSpace(config.Addr) == "" {
		return Config{}, fmt.Errorf("redis address is missing: set REDIS_ADDR or REDIS_HOST (and REDIS_PORT when needed)")
	}

	return config, nil
}

func buildAddr(host string, port int) string {
	cleanHost := strings.TrimSpace(host)
	if cleanHost == "" {
		return ""
	}

	if _, _, err := net.SplitHostPort(cleanHost); err == nil {
		return cleanHost
	}

	if port > 0 {
		return net.JoinHostPort(cleanHost, strconv.Itoa(port))
	}

	return cleanHost
}

func extractHost(addr string) string {
	cleanAddr := strings.TrimSpace(addr)
	if cleanAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(cleanAddr)
	if err == nil {
		return host
	}

	if strings.Count(cleanAddr, ":") == 1 {
		parts := strings.Split(cleanAddr, ":")
		return parts[0]
	}

	return cleanAddr
}

func EnvString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func EnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
