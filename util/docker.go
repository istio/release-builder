package util

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/pkg/errors"
	"istio.io/pkg/log"
)

func Docker(image string, wd string, cmd ...string) error {
	u, err := user.Current()
	if err != nil {
		return errors.Wrap(err, "failed to get user")
	}
	args := []string{
		"run",
		"-t",
		"--sig-proxy=true",
		"-u", u.Uid,
		"--rm",
		"--privileged",
		"--mount", fmt.Sprintf("type=bind,source=%s,destination=/work", wd),
		"-w", "/work",
		"-e", "GOPATH=/work",
		"--entrypoint", "",
		image,
	}
	args = append(args, cmd...)
	log.Infof("Running docker %v", strings.Join(args, " "))
	out, err := exec.Command("docker", args...).CombinedOutput()
	log.Info(string(out))
	return err
}
