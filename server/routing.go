package main

import (
	"os/exec"
)

func ensureIptablesRule(table string, args ...string) {
	checkArgs := []string{}
	if table != "" {
		checkArgs = append(checkArgs, "-t", table)
	}
	checkArgs = append(checkArgs, "-C")
	checkArgs = append(checkArgs, args...)

	if err := exec.Command("iptables", checkArgs...).Run(); err != nil {
		addArgs := []string{}
		if table != "" {
			addArgs = append(addArgs, "-t", table)
		}
		addArgs = append(addArgs, "-A")
		addArgs = append(addArgs, args...)
		exec.Command("iptables", addArgs...).Run()
	}
}
