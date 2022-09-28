package ctrl

import (
	"log"
	"os"
	"os/exec"
)

var services = []string{
	"execution-node",
	"execution-node-2",
	"beacon-node",
	"beacon-node-follower",
	"validator-node",
	"jaeger-tracing",
}

func Run(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("cmd.Start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return exiterr
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	}
	return nil
}

func StartDevnet() error {
	return Run(exec.Command("/bin/sh", "-c", "docker-compose up -d"))
}

func StopDevnet() error {
	return Run(exec.Command("/bin/sh", "-c", "docker-compose down -v"))
}

func RestartDevnet() error {
	if err := StopDevnet(); err != nil {
		return err
	}
	return StartDevnet()
}
