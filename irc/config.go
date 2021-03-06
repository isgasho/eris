package irc

import (
	"errors"
	"io/ioutil"
	"log"
	"sync"

	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
)

type PassConfig struct {
	Password string
}

type TLSConfig struct {
	Key  string
	Cert string
}

func (conf *PassConfig) PasswordBytes() []byte {
	bytes, err := DecodePassword(conf.Password)
	if err != nil {
		log.Fatal("decode password error: ", err)
	}
	return bytes
}

type Config struct {
	sync.Mutex
	filename string

	Network struct {
		Name string
	}

	Server struct {
		PassConfig  `yaml:",inline"`
		Listen      []string
		TLSListen   map[string]*TLSConfig
		Log         string
		MOTD        string
		Name        string
		Description string
	}

	Operator map[string]*PassConfig
	Account  map[string]*PassConfig
}

func (conf *Config) Operators() map[Name][]byte {
	operators := make(map[Name][]byte)
	for name, opConf := range conf.Operator {
		operators[NewName(name)] = opConf.PasswordBytes()
	}
	return operators
}

func (conf *Config) Accounts() map[string][]byte {
	accounts := make(map[string][]byte)
	for name, account := range conf.Account {
		accounts[name] = []byte(account.Password)
	}
	return accounts
}

func (conf *Config) Name() string {
	return conf.filename
}

func (conf *Config) Reload() error {
	conf.Lock()
	defer conf.Unlock()

	newconf, err := LoadConfig(conf.filename)
	if err != nil {
		return nil
	}

	err = mergo.MergeWithOverwrite(conf, newconf)
	if err != nil {
		return nil
	}

	return nil
}

func LoadConfig(filename string) (config *Config, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	config.filename = filename

	if config.Network.Name == "" {
		return nil, errors.New("Network name missing")
	}

	if config.Server.Name == "" {
		return nil, errors.New("Server name missing")
	}

	if !IsHostname(config.Server.Name) {
		return nil, errors.New("Server name must match the format of a hostname")
	}

	if len(config.Server.Listen)+len(config.Server.TLSListen) == 0 {
		return nil, errors.New("Server listening addresses missing")
	}

	return config, nil
}
