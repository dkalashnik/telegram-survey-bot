package questions

import (
	"fmt"
	"strings"
	"sync"
	"telegramsurveylog/pkg/config"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]QuestionStrategy)

	validatorOnce sync.Once
	builtinsOnce  sync.Once
)

// RegisterBuiltins wires config validation to the registry and registers built-in strategies.
func RegisterBuiltins() {
	builtinsOnce.Do(func() {
		registerValidator()
		registerStrategy(NewTextStrategy())
		registerStrategy(NewButtonsStrategy())
	})
}

func registerValidator() {
	validatorOnce.Do(func() {
		config.RegisterQuestionValidator(func(sectionID string, question config.QuestionConfig) error {
			strat := Get(question.Type)
			if strat == nil {
				return fmt.Errorf("config validation failed: question '%s' in section '%s' has unknown type '%s'", question.ID, sectionID, question.Type)
			}
			return strat.Validate(sectionID, question)
		})
	})
}

func registerStrategy(strategy QuestionStrategy) {
	if strategy == nil {
		panic("cannot register nil strategy")
	}
	MustRegister(strategy)
}

// MustRegister adds a strategy to the registry, panicking when a duplicate type is registered.
func MustRegister(strategy QuestionStrategy) {
	if strategy == nil {
		panic("cannot register nil strategy")
	}

	key := normalize(strategy.Name())
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[key]; exists {
		panic(fmt.Sprintf("question strategy '%s' already registered", strategy.Name()))
	}

	registry[key] = strategy
}

// Get returns the strategy for the given type, or nil when absent.
func Get(name string) QuestionStrategy {
	key := normalize(name)
	registryMu.RLock()
	defer registryMu.RUnlock()

	return registry[key]
}

// MustGet returns the registered strategy, panicking when it is missing.
func MustGet(name string) QuestionStrategy {
	strat := Get(name)
	if strat == nil {
		panic(fmt.Sprintf("question strategy '%s' is not registered", name))
	}
	return strat
}

func normalize(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

// resetRegistryForTests wipes registration state. Only used inside unit tests.
func resetRegistryForTests() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]QuestionStrategy)
	validatorOnce = sync.Once{}
	builtinsOnce = sync.Once{}
}
