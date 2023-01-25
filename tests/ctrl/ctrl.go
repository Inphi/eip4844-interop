package ctrl

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

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

// guards against concurrent access to the docker daemon
var dockerMutex sync.Mutex

func StartServices(svcs ...string) error {
	dockerMutex.Lock()
	defer dockerMutex.Unlock()

	svcArg := strings.Join(svcs, " ")
	log.Printf("starting services %s", svcArg)
	err := Run(exec.Command("/bin/sh", "-c", fmt.Sprintf("docker compose up -d %s", svcArg)))
	if err != nil && err.(*exec.ExitError).ExitCode() == 127 {
		err = Run(exec.Command("/bin/sh", "-c", fmt.Sprintf("docker compose up -d %s", svcArg)))
	}
	return err
}

func StopService(svc string) error {
	err := Run(exec.Command("/bin/sh", "-c", fmt.Sprintf("docker compose stop %s", svc)))
	if err != nil && err.(*exec.ExitError).ExitCode() == 127 {
		err = Run(exec.Command("/bin/sh", "-c", fmt.Sprintf("docker compose stop %s", svc)))
	}
	return err
}

func StopDevnet() error {
	err := Run(exec.Command("/bin/sh", "-c", "docker compose down -v"))
	if err != nil && err.(*exec.ExitError).ExitCode() == 127 {
		err = Run(exec.Command("/bin/sh", "-c", "docker compose down -v"))
	}
	return err
}
