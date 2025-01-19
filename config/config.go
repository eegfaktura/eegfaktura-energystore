package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
)

func ReadConfig(path string) {
	viper.SetConfigName("config")
	viper.AddConfigPath(path)
	viper.AutomaticEnv()
	viper.SetConfigType("yml")
	if err := viper.ReadInConfig(); err != nil {
		glog.Exitf("Error reading config file, %s", err)
	}
}

func ReadCertificate(privteKeyFile, publicKeyFile string) (*rsa.PublicKey, *rsa.PrivateKey, error) {
	prvData, err := RSAKeyFile(privteKeyFile)
	if err != nil {
		return nil, nil, err
	}
	prv, err := RSAPrivateKey(prvData)
	if err != nil {
		return nil, nil, err
	}

	pubData, err := RSAKeyFile(publicKeyFile)
	if err != nil {
		return nil, nil, err
	}
	pub, err := RSAPublicKey(pubData)
	if err != nil {
		return nil, nil, err
	}

	return pub, prv, nil
}

func RSAKeyFile(file string) (data []byte, err error) {

	log.Printf("Key-File: %s", file)
	data, err = ioutil.ReadFile(file)
	return
}

func RSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	input := pemDecode(data)

	var err error
	var key interface{}

	if key, err = x509.ParsePKCS1PrivateKey(input); err != nil {
		if key, err = x509.ParsePKCS8PrivateKey(input); err != nil {
			return nil, err
		}
	}

	return key.(*rsa.PrivateKey), nil
}

// RSAPublicKey parses data as *rsa.PublicKey
func RSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	input := pemDecode(data)

	var err error
	var key interface{}

	if key, err = x509.ParsePKIXPublicKey(input); err != nil {
		if cert, err := x509.ParseCertificate(input); err == nil {
			key = cert.PublicKey
		} else {
			return nil, err
		}
	}

	return key.(*rsa.PublicKey), nil
}

func pemDecode(data []byte) []byte {
	if block, _ := pem.Decode(data); block != nil {
		return block.Bytes
	}

	return data
}
