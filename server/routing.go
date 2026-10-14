package main

import (
	"os/exec"
)

func ensureFirewallRule(cmdName, table string, args ...string) {
	checkArgs := []string{}
	if table != "" {
		checkArgs = append(checkArgs, "-t", table)
	}
	checkArgs = append(checkArgs, "-C")
	checkArgs = append(checkArgs, args...)

	if err := exec.Command(cmdName, checkArgs...).Run(); err != nil {
		addArgs := []string{}
		if table != "" {
			addArgs = append(addArgs, "-t", table)
		}
		addArgs = append(addArgs, "-A")
		addArgs = append(addArgs, args...)
		exec.Command(cmdName, addArgs...).Run()
	}
}

func ensureIptablesRule(table string, args ...string) {
	ensureFirewallRule("iptables", table, args...)
}

func ensureIp6tablesRule(table string, args ...string) {
	ensureFirewallRule("ip6tables", table, args...)
}
