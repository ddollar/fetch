package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Cmd struct {
	Dir     string
	Command string
	Env     map[string]string
	Output  io.Writer
}

func Execute(cmd Cmd) {
	wrapped := []string{"/bin/bash", "-c", cmd.Command}
	command := exec.Command(wrapped[0], wrapped[1:]...)
	if cmd.Dir != "" {
		command.Dir = cmd.Dir
	}
	if cmd.Env != nil {
		for key, val := range cmd.Env {
			command.Env = append(command.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	if cmd.Output != nil {
		command.Stdout = cmd.Output
		command.Stderr = cmd.Output
	}
	command.Start()
	command.Wait()
	if !command.ProcessState.Success() {
		fmt.Printf("ERROR: %s\n", cmd.Command)
		os.Exit(100)
	}
}
