package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config — worker uchun barcha sozlamalar. Qiymatlar env'dan (backend bilan bir
// xil nomlar) yoki configs/config.yaml'dan o'qiladi; env ustun.
type Config struct {
	DatabaseURL string `mapstructure:"database_url"`
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
	return &cfg, nil
}
