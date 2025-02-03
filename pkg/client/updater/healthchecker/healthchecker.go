package healthchecker

import (
	log "github.com/sirupsen/logrus"
	"os/exec"
)

type HealthChecker interface {
	HealthCheck() error
}

type shellHealthChecker struct {
	cmd []string
}

func (s *shellHealthChecker) HealthCheck() error {
	if len(s.cmd) == 0 {
		log.Debug("no command to execute, assuming healthy")
		return nil
	}
	return exec.Command(s.cmd[0], s.cmd[1:]...).Run()
}

func NewShellHealthChecker(cmd []string) HealthChecker {
	return &shellHealthChecker{
		cmd: cmd,
	}
}
