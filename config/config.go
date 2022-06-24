package config

import (
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	DefaultLogLevel      = "debug"
	DefaultJWTSecretPath = "secret.pem"
)

// Config holds parsed config params from YAML by Viper
type Config struct {
	Log struct {
		Pretty bool   `yaml:"pretty"`
		Level  string `yaml:"level"`
	} `yaml:"log"`
	ServerName string `yaml:"server_name"`
	RabbitMQ   struct {
		URI             string `yaml:"uri"`
		ChannelPoolSize int    `yaml:"channel_pool_size"`
	} `yaml:"rabbit_mq"`
	Paper struct {
		Binance struct {
			RESTHost string `yaml:"rest_host"`
			WSHost   string `yaml:"ws_host"`
		} `yaml:"binance"`
	} `yaml:"paper"`
	Fees struct {
		Binance        Fees `yaml:"binance"`
		BinanceFutures Fees `yaml:"binance_futures"`
		Kraken         Fees `yaml:"kraken"`
		BybitSpot      Fees `yaml:"bybit_spot"`
		BybitUSDTPerp  Fees `yaml:"bybit_usdt_perp"`
		Coinbase       Fees `yaml:"coinbase"`
		Huobi          Fees `yaml:"huobi"`
		Okex           Fees `yaml:"okex"`
		HitBTC         Fees `yaml:"hitbtc"`
		BinanceUS      Fees `yaml:"binance_us"`
	} `yaml:"fees"`
	Proxy []struct {
		Auth  string   `yaml:"auth"`
		Hosts []string `yaml:"hosts"`
	} `yaml:"proxy"`
	DB struct {
		Log                 bool   `yaml:"log"`
		DisableTLS          bool   `yaml:"disable_tls"`
		User                string `yaml:"user"`
		Password            string `yaml:"password"`
		Host                string `yaml:"host"`
		Name                string `yaml:"name"`
		CertPath            string `yaml:"cert_path"`
		MaxOpenConns        int    `yaml:"max_open_conns"`
		MaxConnLifeTimeMins int    `yaml:"max_conn_life_time_mins"`
	} `yaml:"db"`
	Redis struct {
		Host     string `yaml:"host"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	JWTSecretPath string `yaml:"jwt_secret_path"`
}

// Fees holds configurable info of exchange's maker/taker commission
type Fees struct {
	Maker float64 `json:"maker"`
	Taker float64 `json:"taker"`
}

// Load loads config from YAML by means of Viper lib and injects into Config
func Load(configPath string) (*Config, error) {
	viper.SetConfigName("yaml")

	// Default
	viper.SetDefault("trade", true)
	viper.SetDefault("log.level", DefaultLogLevel)
	viper.SetDefault("bolt.live", true)
	viper.SetDefault("db.live", true)
	viper.SetDefault("jwt_secret_path", DefaultJWTSecretPath)

	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, errors.Errorf("Can't read config file: %s", err)
		}
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file %s changed. Exiting.", e.Name)
		os.Exit(1)
	})

	var cfg Config
	if err := viper.Unmarshal(&cfg, func(decoderCfg *mapstructure.DecoderConfig) {
		decoderCfg.TagName = "yaml"
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal config file %s into %T", configPath, cfg)
	}

	return &cfg, nil
}
