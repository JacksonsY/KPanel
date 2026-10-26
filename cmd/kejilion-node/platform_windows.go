//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster/sshlogin"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"github.com/kejilion/kejilion-panel/internal/windowsnode"
)

var defaultConfigPath = windowsnode.ConfigPath()
var defaultTerminalConfigPath = windowsnode.TerminalConfigPath()
var updateHealthPath = filepath.Join(windowsnode.DataDir(), "update-status.json")
var nodeCheckStatusPath = windowsnode.RuntimePath("check-status.json")
var windowsServiceContext context.Context

func nodeSignalContext() (context.Context, context.CancelFunc) {
	if windowsServiceContext != nil {
		return context.WithCancel(windowsServiceContext)
	}
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
func requireEnrollmentIdentity() error {
	if !windowsnode.IsSystem() {
		return errors.New("Windows enrollment requires the SYSTEM bootstrap service; use the verified installer")
	}
	return nil
}
func requireBrokerIdentity() error {
	if !windowsnode.IsSystem() {
		return errors.New("Windows privileged broker requires LocalSystem")
	}
	return nil
}
func requireEnrollmentCenter(headers http.Header) error {
	if !hasResponseCapability(headers, "windows-command-v1") {
		return errors.New("center does not support Windows nodes; upgrade the center before enrollment")
	}
	return nil
}
func enrollmentCapabilities(value string) ([]string, error) {
	if value == "" {
		if err := terminal.Available(); err != nil {
			return nil, err
		}
		return []string{"monitoring", "terminal", "files"}, nil
	}
	result := []string{"monitoring"}
	for _, c := range strings.Split(value, ",") {
		c = strings.TrimSpace(c)
		if !slices.Contains([]string{"monitoring", "terminal", "files"}, c) {
			return nil, errors.New("unknown Windows node capability")
		}
		if !slices.Contains(result, c) {
			result = append(result, c)
		}
	}
	if slices.Contains(result, "terminal") && terminal.Available() != nil {
		return nil, errors.New("this Windows version does not support ConPTY")
	}
	return result, nil
}

func platformTerminalEnabled(config nodeConfig) bool {
	return slices.Contains(config.Capabilities, "terminal")
}

func installationCapabilities(request windowsInstallRequest) ([]string, error) {
	return enrollmentCapabilities(request.Capabilities)
}
func requireWindowsCapability(parent context.Context, config nodeConfig, capability string) error {
	if !slices.Contains(config.Capabilities, capability) {
		return errors.New("Windows node capability was not enabled at installation")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	secret, err := base64.RawURLEncoding.DecodeString(config.ReportingKey)
	if err != nil || len(secret) != 32 {
		return errors.New("invalid node reporting key")
	}
	defer clear(secret)
	const path = "/api/v3/federation/light/capabilities"
	body := []byte("{}")
	headers, err := signedLightNodeHeaders(config, path, body, secret, nil)
	if err != nil {
		return err
	}
	httpClient, err := nodeHTTPClientForOrigin(config.Origin, config.TLSPublicKeyPin)
	if err != nil {
		return err
	}
	var response struct {
		Capabilities []string `json:"capabilities"`
	}
	if _, _, err = postRawJSONWithHTTPClient(ctx, httpClient, config.Origin+path, body, headers, &response); err != nil {
		return err
	}
	if !slices.Contains(response.Capabilities, "windows-command-v1") {
		return errors.New("center does not support Windows nodes")
	}
	return nil
}

func applyPlatformReport(payload *reportRequest, config nodeConfig) { payload.Platform = "windows" }
func validatePlatformConfig(file *os.File, secret bool) error {
	access := windowsnode.TelemetryRead
	if secret {
		access = windowsnode.SystemOnly
	}
	return windowsnode.ValidateFile(file, access)
}
func writePlatformConfig(path string, value any, secret bool) (bool, error) {
	if !windowsnode.IsSystem() {
		return true, errors.New("Windows configuration writes require SYSTEM")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return true, err
	}
	access := windowsnode.TelemetryRead
	if secret {
		access = windowsnode.SystemOnly
	}
	return true, windowsnode.WriteAtomic(path, append(data, '\n'), access)
}
func repairLegacyNodeConfigAccess() error { return nil }
func preserveNodeConfigAccess(*os.File, os.FileInfo) error {
	return errors.New("Windows atomic writer must use an explicit DACL")
}
func rootOwned(os.FileInfo) bool     { return false }
func processOwned(os.FileInfo) bool  { return false }
func platformStateDirectory() string { return filepath.Join(windowsnode.DataDir(), "state") }
func readUpdateHealthFile(path string) ([]byte, error) {
	return windowsnode.ReadFile(path, maxUpdateHealthBytes, windowsnode.TelemetryRead)
}
func readTrustedRuntimeFile(path string, limit int64) ([]byte, error) {
	return windowsnode.ReadFile(path, limit, windowsnode.TelemetryRead)
}
func publishNodeCheckStatus(summary contract.ServiceCheckSummary) error {
	if !windowsnode.IsSystem() {
		return os.ErrPermission
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return windowsnode.WriteAtomic(nodeCheckStatusPath, data, windowsnode.TelemetryRead)
}
func publishProcdHealthSnapshot(contract.LightNodeHealth) error {
	return errors.New("procd is unavailable on Windows")
}

func latestPlatformLogin(context.Context, *sshlogin.Reader) (*contract.SSHLoginEvent, error) {
	return nil, nil
}
