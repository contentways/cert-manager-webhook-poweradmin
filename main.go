// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

// Command webhook is a cert-manager DNS01 webhook solver for Poweradmin.
package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"k8s.io/klog/v2"

	"github.com/contentways/cert-manager-webhook-poweradmin/internal/solver"
)

// version is set at build time.
var version = "dev"

func main() {
	groupName := os.Getenv("GROUP_NAME")
	if groupName == "" {
		klog.ErrorS(nil, "GROUP_NAME must be set to the API group of the webhook, e.g. acme.contentways.org")
		os.Exit(1)
	}
	klog.InfoS("starting cert-manager-webhook-poweradmin", "version", version, "groupName", groupName)
	cmd.RunWebhookServer(groupName, solver.New())
}
