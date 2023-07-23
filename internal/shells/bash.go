package shells

import (
	"bufio"
	"bytes"
	"fmt"
	// "io"
	"os"
	"os/exec"
	"strings"

	// "github.com/creack/pty"

	"github.com/Azure/InnovationEngine/internal/logging"
	"github.com/Azure/InnovationEngine/internal/utils"
)

// Location where the environment state from commands is captured and sent to
// for being able to share state across commands.
var environmentStateFile = "/tmp/env.txt"

func loadEnvFile(path string) (map[string]string, error) {
	if !utils.FileExists(path) {
		return nil, fmt.Errorf("env file '%s' does not exist", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file '%s': %w", path, err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	env := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			env[parts[0]] = parts[1]
		}
	}
	return env, nil
}

// Resets the stored environment variables file.
func ResetStoredEnvironmentVariables() error {
	return os.Remove(environmentStateFile)
}

type CommandOutput struct {
	StdOut string
	StdErr string
}

// Executes a bash command and returns the output or error.
func ExecuteBashCommand(command string, env map[string]string, inherit_environment_variables bool, forward_input_output bool) (CommandOutput, error) {
	var commandWithStateSaved = []string{
		command,
		"env > /tmp/env.txt",
	}
	var commandToExecute *exec.Cmd

	if !forward_input_output {
		commandToExecute = exec.Command("bash", "-c", strings.Join(commandWithStateSaved, "\n"))
	} else {
		commandToExecute = exec.Command("bash", "-c", command)
	}

	var stdoutBuffer, stderrBuffer bytes.Buffer

	if forward_input_output {
		commandToExecute.Stdout = os.Stdout
		commandToExecute.Stderr = os.Stderr
		commandToExecute.Stdin = os.Stdin
	} else {
		// Capture std out and std err as separate buffers.
		commandToExecute.Stdout = &stdoutBuffer
		commandToExecute.Stderr = &stderrBuffer
	}

	if inherit_environment_variables {
		commandToExecute.Env = os.Environ()
	}

	// Sharing environment variable state between isolated shell executions is a
	// bit tough, but how we handle it is by storing the environment variables
	// after a command is executed within a file and then loading that file
	// before executing the next command. This allows us to share state between
	// isolated command calls.
	envFromPreviousStep, err := loadEnvFile(environmentStateFile)
	if err == nil {
		merged := utils.MergeMaps(env, envFromPreviousStep)
		for k, v := range merged {
			commandToExecute.Env = append(commandToExecute.Env, fmt.Sprintf("%s=%s", k, v))
		}
	} else {
		for k, v := range env {
			commandToExecute.Env = append(commandToExecute.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	logging.GlobalLogger.Tracef("Environment variables: %v", commandToExecute.Env)
	// Execute command, handle errors, and return output.

	err = commandToExecute.Run()
	if forward_input_output {
		return CommandOutput{}, err
	}

	standardOutput, standardError := stdoutBuffer.String(), stderrBuffer.String()

	if err != nil {
		return CommandOutput{}, fmt.Errorf("command exited with '%w' and the message '%s'", err, standardError)
	}

	return CommandOutput{
		StdOut: standardOutput,
		StdErr: standardError,
	}, nil
}
