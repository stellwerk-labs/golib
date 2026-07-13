package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

type CommandRunner struct{}

func (c *CommandRunner) Run(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running %s %v: %w, output: %s", name, arg, err, string(out))
	}

	return out, nil
}

type Runner interface {
	Run(name string, arg ...string) ([]byte, error)
}

func run(args []string, r Runner) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: gittag library1 version")
	}

	lastIndex := len(args) - 1
	libraries := args[:lastIndex]
	version := args[lastIndex]

	fmt.Println("Bumping to version", version)
	fmt.Println("Libraries", libraries)

	for _, lib := range libraries {
		if _, err := r.Run("go", "list", fmt.Sprintf("./%s", lib)); err != nil {
			return fmt.Errorf("validating library %s: %w", lib, err)
		}

		tag := fmt.Sprintf("%s/%s", lib, version)
		fmt.Println("Tagging", tag)
		if _, err := r.Run("git", "tag", "-m", tag, tag); err != nil {
			return err
		}
	}

	if _, err := r.Run("git", "push", "--tags"); err != nil {
		return err
	}

	return nil
}

func main() {
	r := &CommandRunner{}
	if err := run(os.Args[1:], r); err != nil {
		log.Fatal(err)
	}
}
