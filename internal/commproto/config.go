package commproto

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

type ClientConfiguration struct {
	HostAddress     string                          `json:"host-addr"`
	AcceptsCommands bool                            `json:"accepts-commands"`
	TimeServer      *TimeConfiguration              `json:"time-server"`
	TimeClient      *TimeConfiguration              `json:"time-client"`
	Partners        map[string]PartnerConfiguration `json:"partners"`
}

type TimeConfiguration struct {
	Address    string `json:"addr"`
	Passphrase string `json:"passphrase"`
}

type PartnerConfiguration struct {
	Key        ConfigurationKey `json:"key"`
	Passphrase string           `json:"passphrase"`
}

type ConfigurationKey []byte

func (key ConfigurationKey) MarshalJSON() ([]byte, error) {
	if key == nil {
		return []byte("null"), nil
	}
	len := 2 + hex.EncodedLen(len(key))
	result := make([]byte, len)
	result[0] = '"'
	hex.Encode(result[1:], key)
	result[len-1] = '"'
	return result, nil
}

func (key *ConfigurationKey) UnmarshalJSON(data []byte) error {
	if key == nil {
		return errors.New("commproto.ConfigurationKey: UnmarshalJSON on nil pointer")
	}
	if string(data) == "null" {
		*key = nil
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("cannot unmarshal non-string value into ConfigurationKey")
	}
	tmp := make([]byte, hex.DecodedLen(len(data)-2))
	_, err := hex.Decode(tmp, data[1:len(data)-1])
	if err != nil {
		return fmt.Errorf("cannot unmarshal ConfigurationKey: %v", err)
	}
	*key = tmp
	return nil
}

func (config *ClientConfiguration) Validate() error {
	if config.HostAddress == "" {
		return errors.New("missing 'host-addr'")
	}

	if config.TimeServer != nil && config.TimeClient != nil {
		return errors.New("cannot have both 'time-sever' and 'time-client'")
	}

	if config.TimeServer != nil {
		if err := config.TimeServer.Validate(); err != nil {
			return fmt.Errorf("in 'time-sever': %v", err)
		}
	}

	if config.TimeClient != nil {
		if err := config.TimeClient.Validate(); err != nil {
			return fmt.Errorf("in 'time-client': %v", err)
		}
	}

	for name, partner := range config.Partners {
		if len(partner.Key) == 0 {
			return fmt.Errorf("missing 'key' for partner '%s'", name)
		}
		if len(partner.Key) != 16 {
			return fmt.Errorf("'key' for partner '%s' has wrong length (expected 16 but was %d)", name, len(partner.Key))
		}
		if partner.Passphrase == "" {
			return fmt.Errorf("missing 'passphrase' for partner '%s'", name)
		}
	}

	return nil
}

func (config *TimeConfiguration) Validate() error {
	if config.Address == "" {
		return errors.New("missing 'addr'")
	}
	if config.Passphrase == "" {
		return errors.New("missing 'passphrase'")
	}
	return nil
}

func ParseConfiguration(filename string) (*ClientConfiguration, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config ClientConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("config file '%s': %v", filename, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config file '%s': %v", filename, err)
	}

	return &config, nil
}
