package config

import (
	"os"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type Config struct {
	DB        DBConfig
	JWTSecret string
}

func LoadConfig() Config {
	devModeEnabled := os.Getenv("DEV")
	var dbName string

	if devModeEnabled != "" {
		dbName = os.Getenv("DEV_POSTGRES_DB")
	} else {
		dbName = os.Getenv("PROD_POSTGRES_DB")
	}

	return Config{
		DB: DBConfig{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Name:     dbName,
		},
		JWTSecret: os.Getenv("JWT_SECRET"),
	}

}
