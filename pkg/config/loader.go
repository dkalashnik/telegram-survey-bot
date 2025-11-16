package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	loadedConfig *RecordConfig

	configMutex sync.RWMutex
)

func LoadConfig(filePath string) error {
	log.Printf("Loading configuration from %s...", filePath)

	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	var cfg RecordConfig

	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	configMutex.Lock()
	loadedConfig = &cfg
	configMutex.Unlock()

	log.Printf("Configuration loaded and validated successfully. %d sections found.", len(loadedConfig.Sections))
	return nil
}

func GetConfig() *RecordConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if loadedConfig == nil {
		log.Println("Warning: GetConfig() called before configuration was loaded.")
	}
	return loadedConfig
}
