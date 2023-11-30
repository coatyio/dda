// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package config defines DDA configuration types to be used programmatically or
// within a DDA configuration file in YAML format.
//
// Note that the fields of all configuration types are not documented in code
// but solely in the default [DDA YAML] configuration file located in the
// project root folder (single source of truth).
//
// [DDA YAML]: https://github.com/coatyio/dda/blob/main/dda.yaml
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const configSchemaVersion = "1"

var compliesWithNamingConvention = regexp.MustCompile("^[-_.0-9a-zA-Z]+$").MatchString

// Asserts that the given name conforms to the DDA naming convention for
// configuration items used as pub-sub topic fields. Such a name must be
// non-empty and only contain ASCII digits 0-9, lowercase letters a-z, uppercase
// letters A-Z, dot (.), hyphen (-), or underscore (_). Returns an error if
// assertion fails; nil otherwise.
//
// This function is intended to be used by communication binding
// implementations.
func ValidateName(name string, context ...string) error {
	if !compliesWithNamingConvention(name) {
		return fmt.Errorf(`invalid %s "%s": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`, strings.Join(context, " "), name)
	}
	return nil
}

// Asserts that the given value is a non-empty string. Returns an error if
// assertion fails; nil otherwise. The given context strings are embedded into
// the error.
func ValidateNonEmpty(value string, context ...string) error {
	if value == "" {
		return fmt.Errorf(`invalid %s "%s": must not be empty`, strings.Join(context, " "), value)
	}
	return nil
}

// A Config represents the complete hierarchy of configuration parameters for
// all DDA services as nested struct types that map to the underlying YAML
// configuration schema. It should be created with New() or ReadConfig() to
// ensure all fields are correctly populated.
type Config struct {
	Version  string
	Identity Identity
	Cluster  string
	Apis     ConfigApis
	Services ConfigServices
}

// Verify asserts that the configuration version matches the supported
// configuration schema and that the Cluster name is valid. Returns an error
// otherwise.
func (c *Config) Verify() error {
	if c.Version != configSchemaVersion {
		return fmt.Errorf(`incompatible configuration version "%s", requiring version "%s"`, c.Version, configSchemaVersion)
	}

	if err := ValidateName(c.Cluster, "configuration cluster"); err != nil {
		return err
	}

	return nil
}

// An Identity represents the unique identity of a DDA.
type Identity struct {
	Name string
	Id   string
}

// ConfigApis comprises server-side configuration options of all public DDA
// Client APIs.
type ConfigApis struct {
	Grpc    ConfigApi
	GrpcWeb ConfigWebApi `yaml:"grpc-web"`
	Cert    string
	Key     string
}

// A ConfigApi comprises server-side configuration options of a specific public
// DDA Client API.
type ConfigApi struct {
	Address   string
	Disabled  bool
	Keepalive time.Duration
}

// A ConfigApi used by Web HTTP clients.
type ConfigWebApi struct {
	Address                  string
	Disabled                 bool
	AccessControlAllowOrigin []string `yaml:"access-control-allow-origin"`
}

// ConfigServices provides configuration options for all peripheral DDA
// services.
type ConfigServices struct {
	Com   ConfigComService
	Store ConfigStoreService
	// TODO State ConfigStateService
}

// ConfigComService defines configuration options for a selected pub-sub
// messaging protocol.
type ConfigComService struct {
	Protocol string
	Url      string
	Auth     AuthOptions
	Opts     map[string]any
	Disabled bool
}

// AuthOptions defines authentication options for a selected pub-sub
// messaging protocol.
type AuthOptions struct {
	Method   string
	Cert     string
	Key      string
	Verify   bool
	Username string
	Password string
}

// ConfigStoreService provides configuration options for a selected local
// key-value storage.
type ConfigStoreService struct {
	Engine   string
	Location string
	Disabled bool
}

// ReadConfig reads and parses the given DDA configuration file in YAML format
// and returns a *Config on success or an error, otherwise.
func ReadConfig(file string) (*Config, error) {
	c, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return nil, err
	}

	config := New()
	err = yaml.Unmarshal(c, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// New creates a Config struct pre-filled with default values as specified in
// the YAML DDA configuration format.
func New() *Config {
	return &Config{
		Version: configSchemaVersion,
		Identity: Identity{
			Name: "DDA",
			Id:   uuid.NewString(),
		},
		Cluster: "dda",
		Apis: ConfigApis{
			Grpc: ConfigApi{
				Address:   ":8900",
				Disabled:  false,
				Keepalive: 2 * time.Hour,
			},
			GrpcWeb: ConfigWebApi{
				Address:                  ":8800",
				AccessControlAllowOrigin: nil,
				Disabled:                 false,
			},
			Cert: "",
			Key:  "",
		},
		Services: ConfigServices{
			Com: ConfigComService{
				Protocol: "mqtt5",
				Url:      "",
				Auth: AuthOptions{
					Method:   "none",
					Cert:     "",
					Key:      "",
					Verify:   true,
					Username: "",
					Password: "",
				},
				Opts:     make(map[string]any),
				Disabled: false,
			},
			Store: ConfigStoreService{
				Engine:   "pebble",
				Location: "",
				Disabled: true,
			},
		},
	}
}
