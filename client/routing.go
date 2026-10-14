package main

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
)

func cleanRelayHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return strings.Trim(host, "[]")
}

func getGatewayFor(relayIP string) (gateway, iface string, err error) {
	cleanIP := cleanRelayHost(relayIP)

	out, err := exec.Command("ip", "route", "get", cleanIP).Output()
	if err != nil {
		return "", "", fmt.Errorf("ip route get %s failed: %w", cleanIP, err)
	}

	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			gateway = fields[i+1]
		}
		if f == "dev" && i+1 < len(fields) {
			iface = fields[i+1]
		}
	}
	return gateway, iface, nil
}

func setupRoutes(relayIP, tunName, gw, dev string) {
	cleanIP := cleanRelayHost(relayIP)
	parsedIP := net.ParseIP(cleanIP)

	if parsedIP != nil {
		var cmdArgs []string
		if parsedIP.To4() != nil {
			cmdArgs = []string{"route", "add", cleanIP + "/32"}
		} else {
			cmdArgs = []string{"-6", "route", "add", cleanIP + "/128"}
		}

		if gw != "" {
			cmdArgs = append(cmdArgs, "via", gw, "dev", dev)
		} else if dev != "" {
			cmdArgs = append(cmdArgs, "dev", dev)
		}

		if len(cmdArgs) > 3 {
			if err := exec.Command("ip", cmdArgs...).Run(); err != nil {
				log.Printf("Warning: failed to add bypass route for %s: %v", cleanIP, err)
			} else {
				log.Printf("Added bypass route for relay server (%s) via dev %s", cleanIP, dev)
			}
		}
	}

	if err := exec.Command("ip", "route", "add", "0.0.0.0/1", "dev", tunName).Run(); err != nil {
		log.Printf("Warning: failed to add 0.0.0.0/1: %v", err)
	}
	if err := exec.Command("ip", "route", "add", "128.0.0.0/1", "dev", tunName).Run(); err != nil {
		log.Printf("Warning: failed to add 128.0.0.0/1: %v", err)
	}
	log.Println("Installed IPv4 redirect routes (0.0.0.0/1 & 128.0.0.0/1) ->", tunName)
}

func cleanupRoutes(relayIP, tunName, gw, dev string) {
	log.Println("Cleaning up routing table...")

	_ = exec.Command("ip", "route", "del", "0.0.0.0/1", "dev", tunName).Run()
	_ = exec.Command("ip", "route", "del", "128.0.0.0/1", "dev", tunName).Run()

	cleanIP := cleanRelayHost(relayIP)
	parsedIP := net.ParseIP(cleanIP)

	if parsedIP != nil {
		var cmdArgs []string
		if parsedIP.To4() != nil {
			cmdArgs = []string{"route", "del", cleanIP + "/32"}
		} else {
			cmdArgs = []string{"-6", "route", "del", cleanIP + "/128"}
		}

		if gw != "" {
			cmdArgs = append(cmdArgs, "via", gw, "dev", dev)
		} else if dev != "" {
			cmdArgs = append(cmdArgs, "dev", dev)
		}

		if len(cmdArgs) > 3 {
			_ = exec.Command("ip", cmdArgs...).Run()
		}
	}

	log.Println("Routing restored to normal.")
}
