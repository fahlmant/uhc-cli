/*
Copyright (c) 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains the types and functions used to manage the configuration of the command line
// client.

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Config is the type used to store the configuration of the client.
type Config struct {
	AccessToken  string   `json:"access_token,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	Insecure     bool     `json:"insecure,omitempty"`
	Password     string   `json:"password,omitempty"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Token        string   `json:"token,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	URL          string   `json:"url,omitempty"`
	User         string   `json:"user,omitempty"`
}

// Load loads the configuration from the configuration file. If the configuration file doesn't exist
// it will return an empty configuration object.
func Load() (cfg *Config, err error) {
	file, err := Location()
	if err != nil {
		return
	}
	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		cfg = nil
		err = nil
		return
	}
	if err != nil {
		err = fmt.Errorf("can't check if config file '%s' exists: %v", file, err)
		return
	}
	// #nosec G304
	data, err := ioutil.ReadFile(file)
	if err != nil {
		err = fmt.Errorf("can't read config file '%s': %v", file, err)
		return
	}
	cfg = new(Config)
	err = json.Unmarshal(data, cfg)
	if err != nil {
		err = fmt.Errorf("can't parse config file '%s': %v", file, err)
		return
	}
	return
}

// Save saves the given configuration to the configuration file.
func Save(cfg *Config) error {
	file, err := Location()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("can't marshal config: %v", err)
	}
	err = ioutil.WriteFile(file, data, 0600)
	if err != nil {
		return fmt.Errorf("can't write file '%s': %v", file, err)
	}
	return nil
}

// Remove removes the configuration file.
func Remove() error {
	file, err := Location()
	if err != nil {
		return err
	}
	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		return nil
	}
	err = os.Remove(file)
	if err != nil {
		return err
	}
	return nil
}

// Location returns the location of the configuration file.
func Location() (path string, err error) {
	home := os.Getenv("HOME")
	if home == "" {
		err = fmt.Errorf("can't find home directory, HOME environment variable is empty")
		return
	}
	path = filepath.Join(home, ".uhc.json")
	return
}

// Armed checks if the configuration contains either credentials or tokens that haven't expired, so
// that it can be used to perform authenticated requests.
func Armed(cfg *Config) (armed bool, err error) {
	if cfg.User != "" && cfg.Password != "" {
		armed = true
		return
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		armed = true
		return
	}
	now := time.Now()
	if cfg.AccessToken != "" {
		var expires bool
		var left time.Duration
		expires, left, err = tokenExpiry(cfg.AccessToken, now)
		if err != nil {
			return
		}
		if !expires || left > 5*time.Second {
			armed = true
			return
		}
	}
	if cfg.RefreshToken != "" {
		var expires bool
		var left time.Duration
		expires, left, err = tokenExpiry(cfg.RefreshToken, now)
		if err != nil {
			return
		}
		if !expires || left > 10*time.Second {
			armed = true
			return
		}
	}
	return
}

func tokenExpiry(text string, now time.Time) (expires bool, left time.Duration, err error) {
	parser := new(jwt.Parser)
	token, _, err := parser.ParseUnverified(text, jwt.MapClaims{})
	if err != nil {
		err = fmt.Errorf("cant parse token: %v", err)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		err = fmt.Errorf("expected map claims bug got %T", claims)
		return
	}
	claim, ok := claims["exp"]
	if !ok {
		err = fmt.Errorf("token doesn't contain the 'exp' claim")
		return
	}
	exp, ok := claim.(float64)
	if !ok {
		err = fmt.Errorf("expected floating point 'exp' but got %T", claim)
		return
	}
	if exp == 0 {
		expires = false
		left = 0
	} else {
		expires = true
		left = time.Unix(int64(exp), 0).Sub(now)
	}
	return
}
