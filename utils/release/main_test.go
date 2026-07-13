package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockRunner struct {
	commands []string
}

func (m *MockRunner) Run(name string, arg ...string) ([]byte, error) {
	command := []string{name}
	command = append(command, arg...)

	m.commands = append(m.commands, strings.Join(command, " "))

	return nil, nil
}

func TestRun(t *testing.T) {
	assert := assert.New(t)

	m := &MockRunner{}

	assert.NoError(run([]string{"httplogger", "hlogger", "v1.2.3"}, m))
	assert.Equal([]string{
		"go list ./httplogger",
		"git tag -m httplogger/v1.2.3 httplogger/v1.2.3",
		"go list ./hlogger",
		"git tag -m hlogger/v1.2.3 hlogger/v1.2.3",
		"git push --tags",
	}, m.commands)
}
