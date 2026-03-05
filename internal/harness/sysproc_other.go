//go:build !linux && !darwin

package harness

import "os/exec"

func setSysProcAttr(_ *exec.Cmd) {}
