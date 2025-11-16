package questions

import (
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"testing"
)

type fakeStrategy struct {
	name string
}

func (f *fakeStrategy) Name() string { return f.name }
func (f *fakeStrategy) Validate(sectionID string, question config.QuestionConfig) error {
	return nil
}
func (f *fakeStrategy) Render(RenderContext) (PromptSpec, error) {
	return PromptSpec{Text: "prompt"}, nil
}
func (f *fakeStrategy) HandleAnswer(AnswerContext, AnswerInput) (AnswerResult, error) {
	return AnswerResult{Advance: true}, nil
}

func TestMustRegisterPanicsOnDuplicate(t *testing.T) {
	resetRegistryForTests()

	MustRegister(&fakeStrategy{name: "dup"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when registering duplicate strategy")
		}
	}()

	MustRegister(&fakeStrategy{name: "dup"})
}

func TestGetReturnsRegisteredStrategy(t *testing.T) {
	resetRegistryForTests()

	strat := &fakeStrategy{name: "custom"}
	MustRegister(strat)

	got := Get("custom")
	if got == nil || got.Name() != "custom" {
		t.Fatalf("expected to retrieve registered strategy got=%v", got)
	}
}
