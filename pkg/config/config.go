package config

import (
	"fmt"
	"log"
	"sync"
)

type RecordConfig struct {
	Sections map[string]SectionConfig `yaml:"sections"`
	Metadata map[string]string        `yaml:"metadata,omitempty"`
}

type SectionConfig struct {
	Title     string           `yaml:"title"`
	Questions []QuestionConfig `yaml:"questions"`
}

type QuestionConfig struct {
	ID     string `yaml:"id"`
	Prompt string `yaml:"prompt"`

	Type     string         `yaml:"type"`
	StoreKey string         `yaml:"store_key"`
	Options  []ButtonOption `yaml:"options,omitempty"`
}

type ButtonOption struct {
	Text  string `yaml:"text"`
	Value string `yaml:"value"`
}

func (rc *RecordConfig) Validate() error {
	if rc == nil {
		return fmt.Errorf("config is nil")
	}
	if len(rc.Sections) == 0 {
		return fmt.Errorf("config validation failed: no sections defined")
	}

	uniqueStoreKeys := make(map[string]bool)

	for sectionID, section := range rc.Sections {
		if section.Title == "" {
			return fmt.Errorf("config validation failed: section '%s' has no title", sectionID)
		}
		if len(section.Questions) == 0 {

			continue
		}

		for i, question := range section.Questions {
			if question.ID == "" {
				return fmt.Errorf("config validation failed: question #%d in section '%s' has no id", i+1, sectionID)
			}
			if question.Prompt == "" {
				return fmt.Errorf("config validation failed: question '%s' in section '%s' has no prompt", question.ID, sectionID)
			}
			if question.StoreKey == "" {
				return fmt.Errorf("config validation failed: question '%s' in section '%s' has no store_key", question.ID, sectionID)
			}

			if uniqueStoreKeys[question.StoreKey] {
				return fmt.Errorf("config validation failed: duplicate store_key '%s' found (in question '%s', section '%s')", question.StoreKey, question.ID, sectionID)
			}
			uniqueStoreKeys[question.StoreKey] = true

			if err := validateQuestionWithStrategy(sectionID, question); err != nil {
				return err
			}
		}
	}
	return nil
}

type QuestionValidator func(sectionID string, question QuestionConfig) error

var (
	questionValidator QuestionValidator
	validatorMu       sync.RWMutex
)

func RegisterQuestionValidator(fn QuestionValidator) {
	validatorMu.Lock()
	defer validatorMu.Unlock()
	questionValidator = fn
}

func validateQuestionWithStrategy(sectionID string, question QuestionConfig) error {
	fn := currentValidator()
	if fn == nil {
		switch question.Type {
		case "text":
			if len(question.Options) > 0 {
				log.Printf("Warning: question '%s' in section '%s' is type 'text' but has options defined", question.ID, sectionID)
			}
			return nil
		case "buttons":
			if len(question.Options) == 0 {
				return fmt.Errorf("config validation failed: question '%s' in section '%s' is type 'buttons' but has no options", question.ID, sectionID)
			}
			for j, option := range question.Options {
				if option.Text == "" {
					return fmt.Errorf("config validation failed: option #%d for question '%s' in section '%s' has no text", j+1, question.ID, sectionID)
				}
				if option.Value == "" {
					return fmt.Errorf("config validation failed: option #%d for question '%s' in section '%s' has no value", j+1, question.ID, sectionID)
				}
			}
			return nil
		default:
			return fmt.Errorf("config validation failed: question '%s' in section '%s' has unknown type '%s'", question.ID, sectionID, question.Type)
		}
	}
	return fn(sectionID, question)
}

func currentValidator() QuestionValidator {
	validatorMu.RLock()
	defer validatorMu.RUnlock()
	return questionValidator
}
