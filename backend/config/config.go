package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	AWS      AWSConfig
	JWT      JWTConfig
	Media    MediaConfig
}

type AppConfig struct {
	Env  string
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	BucketName      string
	CDNBaseURL      string
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type MediaConfig struct {
	WatermarkEnabled  bool
	MaxImageSizeBytes int64
	MaxVideoSizeBytes int64
	StorageQuotaBytes int64
	WorkerConcurrency int
}

func Load() *Config {
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8080")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("POSTGRES_HOST", "localhost")
	viper.SetDefault("POSTGRES_PORT", 5432)
	viper.SetDefault("POSTGRES_USER", "mediashare")
	viper.SetDefault("POSTGRES_DB", "mediashare")
	viper.SetDefault("POSTGRES_SSL_MODE", "disable")
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("JWT_ACCESS_TTL", "15m")
	viper.SetDefault("JWT_REFRESH_TTL", "168h")
	viper.SetDefault("WATERMARK_ENABLED", false)
	viper.SetDefault("MAX_IMAGE_SIZE_BYTES", int64(10485760))
	viper.SetDefault("MAX_VIDEO_SIZE_BYTES", int64(524288000))
	viper.SetDefault("STORAGE_QUOTA_BYTES", int64(10737418240))
	viper.SetDefault("WORKER_CONCURRENCY", 3)

	accessTTL, _ := time.ParseDuration(viper.GetString("JWT_ACCESS_TTL"))
	refreshTTL, _ := time.ParseDuration(viper.GetString("JWT_REFRESH_TTL"))

	return &Config{
		App: AppConfig{
			Env:  viper.GetString("APP_ENV"),
			Port: viper.GetString("PORT"),
		},
		Database: DatabaseConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetInt("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			DBName:   viper.GetString("POSTGRES_DB"),
			SSLMode:  viper.GetString("POSTGRES_SSL_MODE"),
		},
		Redis: RedisConfig{
			Addr:     viper.GetString("REDIS_ADDR"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		AWS: AWSConfig{
			AccessKeyID:     viper.GetString("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: viper.GetString("AWS_SECRET_ACCESS_KEY"),
			Region:          viper.GetString("AWS_REGION"),
			BucketName:      viper.GetString("S3_BUCKET_NAME"),
			CDNBaseURL:      viper.GetString("CDN_BASE_URL"),
		},
		JWT: JWTConfig{
			Secret:     viper.GetString("JWT_SECRET"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		Media: MediaConfig{
			WatermarkEnabled:  viper.GetBool("WATERMARK_ENABLED"),
			MaxImageSizeBytes: viper.GetInt64("MAX_IMAGE_SIZE_BYTES"),
			MaxVideoSizeBytes: viper.GetInt64("MAX_VIDEO_SIZE_BYTES"),
			StorageQuotaBytes: viper.GetInt64("STORAGE_QUOTA_BYTES"),
			WorkerConcurrency: viper.GetInt("WORKER_CONCURRENCY"),
		},
	}
}
