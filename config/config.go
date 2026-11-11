package config

import (
	"strings"

	"at.ourproject/energystore/internal/logging/glog"
	"github.com/spf13/viper"
)

func ReadConfig(path string) {
	viper.SetConfigName("config")
	viper.AddConfigPath(path)
	viper.AutomaticEnv()
	viper.SetConfigType("yml")

	viper.SetEnvPrefix("ENERGYSTORE")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		glog.Exitf("Error reading config file, %s", err)
	}
}
