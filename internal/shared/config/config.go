package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// Config — worker uchun barcha sozlamalar. Qiymatlar env'dan (backend bilan bir
// xil nomlar) yoki configs/config.yaml'dan o'qiladi; env ustun.
type Config struct {
	DatabaseURL string `mapstructure:"database_url"`
	DBDsn       string `mapstructure:"db_dsn"` // backend database-secret'dagi nom (DATABASE_URL bo'lmasa shu)
	AppDBHost   string `mapstructure:"app_db_host"`
	AppDBPort   string `mapstructure:"app_db_port"`
	AppDBUser   string `mapstructure:"app_db_user"`
	AppDBPass   string `mapstructure:"app_db_password"`
	AppDBName   string `mapstructure:"app_db_name"`
	AppDBSSL    string `mapstructure:"app_db_ssl_mode"`
	RedisURL    string `mapstructure:"redis_url"`
	MongoURL    string `mapstructure:"mongo_url"`

	K8sNamespace  string `mapstructure:"k8s_namespace"`
	PrometheusURL string `mapstructure:"prometheus_url"`

	StorageURL         string `mapstructure:"storage_url"`
	StorageID          string `mapstructure:"storage_id"`
	StorageKey         string `mapstructure:"storage_key"`
	StorageRegion      string `mapstructure:"storage_region"`
	StorageBucketMedia string `mapstructure:"storage_bucket_media"`

	TaskStream        string `mapstructure:"task_stream"`
	TaskConsumerGroup string `mapstructure:"task_consumer_group"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigFile("configs/config.yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Backend .env bilan bir xil env nomlari.
	for key, env := range map[string]string{
		"database_url":         "DATABASE_URL",
		"db_dsn":               "DB_DSN",
		"app_db_host":          "APP_DB_HOST",
		"app_db_port":          "APP_DB_PORT",
		"app_db_user":          "APP_DB_USER",
		"app_db_password":      "APP_DB_PASSWORD",
		"app_db_name":          "APP_DB_NAME",
		"app_db_ssl_mode":      "APP_DB_SSL_MODE",
		"redis_url":            "REDIS_URL",
		"mongo_url":            "MONGO_URL",
		"k8s_namespace":        "K8S_NAMESPACE",
		"prometheus_url":       "PROMETHEUS_URL",
		"storage_url":          "STORAGE_URL",
		"storage_id":           "STORAGE_ID",
		"storage_key":          "STORAGE_KEY",
		"storage_region":       "STORAGE_REGION",
		"storage_bucket_media": "STORAGE_BUCKET_MEDIA",
		"task_stream":          "TASK_STREAM",
		"task_consumer_group":  "TASK_CONSUMER_GROUP",
	} {
		_ = v.BindEnv(key, env)
	}

	v.SetDefault("k8s_namespace", "botmother")
	v.SetDefault("prometheus_url", "http://monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090")
	v.SetDefault("task_stream", "goworker:tasks")
	v.SetDefault("task_consumer_group", "goworker")

	// config.yaml ixtiyoriy — env yetarli bo'lsa fayl shart emas.
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = cfg.DBDsn
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = appDatabaseURL(cfg)
	}
	return &cfg, nil
}

func appDatabaseURL(cfg Config) string {
	if cfg.AppDBHost == "" || cfg.AppDBUser == "" || cfg.AppDBName == "" {
		return ""
	}
	port := cfg.AppDBPort
	if port == "" {
		port = "5432"
	}
	sslMode := cfg.AppDBSSL
	if sslMode == "" {
		sslMode = "disable"
	}

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.AppDBUser, cfg.AppDBPass),
		Host:   fmt.Sprintf("%s:%s", cfg.AppDBHost, port),
		Path:   cfg.AppDBName,
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}
