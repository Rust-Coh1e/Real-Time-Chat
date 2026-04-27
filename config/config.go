package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisPort     string
	Secret        string
	MinioConn     string
	MinioUsername string
	MinioPassword string
	MinioBucket   string
	MailerHost    string
	MailerPort    string
	MailerFrom    string
	IP            string
}

func Load() (*Config, error) {

	err := godotenv.Load("config.env")

	if err != nil {
		return nil, fmt.Errorf("error loading config.env: %w", err)
	}

	return &Config{
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        os.Getenv("DB_PORT"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_NAME"),
		RedisPort:     os.Getenv("REDIS_PORT"),
		Secret:        os.Getenv("SECRET_KEY"),
		MinioConn:     os.Getenv("MINIO_CONN"),
		MinioUsername: os.Getenv("MINIO_USERNAME"),
		MinioPassword: os.Getenv("MINIO_PASSWORD"),
		MinioBucket:   os.Getenv("MINIO_BUCKET"),
		MailerHost:    os.Getenv("MAILER_HOST"),
		MailerPort:    os.Getenv("MAILER_PORT"),
		MailerFrom:    os.Getenv("MAILER_FROM"),
		IP:            os.Getenv("IP"),
	}, nil
}
