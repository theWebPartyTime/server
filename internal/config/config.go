package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
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
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file ", err)
	}
	return Config{
		DB: DBConfig{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Name:     os.Getenv("POSTGRES_DB"),
		},
		JWTSecret: os.Getenv("JWT_SECRET"),
	}

}
