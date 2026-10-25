package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/cluster"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/mcpaccess"
	"github.com/kejilion/kejilion-panel/internal/redact"
)

type mcpTaskReference struct {
	Owner string `json:"owner"`
	ID    string `json:"id"`
}

const mcpTaskDisplayDescription = "Task output uses model-v1: logs are projected from complete bounded owner context before tailing; offset and nextOffset are UTF-8 byte positions in the redacted display snapshot, never raw terminal bytes."

func mcpTaskDisplayRequired(tool string) bool {
	if tool == "host_app_jobs" || tool == "host_web_jobs" {
		return true
	}
	for source, owner := range managedJobSources {
		if owner.Output && tool == "host_"+source+"_job" {
			return true
		}
	}
	return false
}

// Both the target's Agent response and the controller's peer response must
// carry the new display contract. An old peer's redacted=true is insufficient.
func mcpTaskDisplayBody(tool string, body []byte) error {
	if !mcpTaskDisplayRequired(tool) {
		return nil
	}
	var value map[string]any
	if json.Unmarshal(body, &value) != nil {
		return errors.New("task_projection_invalid")
	}
	if inner, ok := value["data"].(map[string]any); ok {
		value = inner
	}
	if value["projection"] != "model-v1" {
		return errors.New("task_projection_upgrade_required")
	}
	return nil
}

// A receipt is not a safe log source: it may already contain an owner-tail or
// a legacy terminal window whose sensitive prefix is gone. Keep its task and
// state, and direct subsequent output reads to the context-qualified owner.
func mcpUnclassifiedLogsProjection(value any) {
	var visit func(any, int)
	visit = func(value any, depth int) {
		if depth > 32 {
			return
		}
		switch item := value.(type) {
		case map[string]any:
			qualified := item["projection"] == "model-v1"
			for key, child := range item {
				if strings.EqualFold(key, "logs") && !qualified {
					item[key] = []string{"[REDACTED: task log context unavailable; read qualified task output]"}
					item["logsContextUnavailable"] = true
					continue
				}
				if strings.EqualFold(key, "dataBase64") {
					item[key] = "[REDACTED: raw terminal output]"
					continue
				}
				if key == "text" && item["nextOffset"] != nil && !qualified {
					item[key] = "[REDACTED: terminal context unavailable]"
					continue
				}
				visit(child, depth+1)
			}
		case []any:
			for _, child := range item {
				visit(child, depth+1)
			}
		}
	}
	visit(value, 0)
}

func mcpDisplayJSON(raw []byte) ([]byte, error) {
	if len(raw) > 8<<20 || !json.Valid(raw) {
		return nil, errors.New("invalid_display_response")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, errors.New("invalid_display_response")
	}
	mcpUnclassifiedLogsProjection(value)
	display, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return redact.JSON(display)
}

var mcpMaintenanceID = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}(Z-[a-f0-9]{8}|\.[0-9]{9}Z)$`)

func mcpTaskID(owner, id string) bool {
	if owner == "system" {
		return mcpMaintenanceID.MatchString(id)
	}
	return ownerJobIDPattern.MatchString(id)
}

// Owner and route are derived locally, never supplied as a URL by a client.
func mcpReceiptTask(tool string, response AgentResponse) *mcpTaskReference {
	var receipt struct {
		ID     string `json:"id"`
		TaskID string `json:"taskId"`
	}
	if json.Unmarshal(response.Body, &receipt) != nil {
		return nil
	}
	if strings.HasPrefix(tool, "host_system_") && mcpTaskID("system", receipt.TaskID) {
		return &mcpTaskReference{Owner: "system", ID: receipt.TaskID}
	}
	if response.StatusCode != 202 || !ownerJobIDPattern.MatchString(receipt.ID) {
		return nil
	}
	owner := ""
	switch {
	case strings.HasPrefix(tool, "host_backup_"):
		owner = "backup"
	case tool == "host_system_partition_action":
		owner = "partition"
	case tool == "host_docker_task":
		owner = "docker"
	case tool == "host_site_create":
		owner = "site"
	case tool == "host_web_action":
		owner = "web"
	case tool == "host_diagnostic_start":
		owner = "diagnostic"
	case tool == "host_file_action":
		owner = "file_archive"
	case strings.HasPrefix(tool, "host_app_"):
		owner = "app"
	}
	if owner == "" {
		return nil
	}
	return &mcpTaskReference{Owner: owner, ID: receipt.ID}
}

func mcpTaskState(data map[string]any) string {
	if result, ok := data["result"].(map[string]any); ok {
		if failures, ok := result["failed"].([]any); ok && len(failures) > 0 {
			return "failed"
		}
	}
	value, _ := data["status"].(string)
	if value == "" {
		value, _ = data["state"].(string)
	}
	switch value {
	case "succeeded", "completed", "ready":
		return "succeeded"
	case "failed", "cancelled", "canceled", "interrupted":
		return "failed"
	case "queued", "running", "waiting_input", "waiting_for_input", "cancelling", "canceling", "accepted":
		return "submitted"
	default:
		return "unknown"
	}
}

func mcpFileFailures(body []byte) bool {
	var result contract.FileActionResult
	return json.Unmarshal(body, &result) == nil && len(result.Failed) > 0
}

func (s *Server) observeMCPOperation(ctx context.Context, call mcpRequest, o mcpaccess.Operation) (mcpaccess.Operation, error) {
	host, err := s.cluster.Host(ctx, o.HostID)
	if err != nil || !s.mcpCallAllowed(call, host) || o.HostIdentity != s.mcpHostIdentity(host) {
		return o, errors.New("host_not_authorized")
	}
	if o.State != "submitted" && o.State != "unknown" {
		return o, nil
	}
	op, ok := managedCatalog()[o.Tool]
	if !ok || !op.allowed(call.principal.Policy) {
		return o, errors.New("operation_not_authorized")
	}
	if !host.IsLocal {
		response, err := s.cluster.ManagedRemote(ctx, host.ID, cluster.ManagedRequest{Version: 1, Mode: "status", FileRoots: call.principal.Policy.FileRoots, ClientID: call.principal.ID, OperationID: o.ID})
		if err != nil {
			return o, errors.New("remote_operation_status_unavailable")
		}
		if response.ErrorCode != "" {
			return o, errors.New(response.ErrorCode)
		}
		state, result, err := mcpRemoteOperationResult(response)
		if err != nil {
			return o, err
		}
		current, err := s.cluster.Host(ctx, host.ID)
		if err != nil || !s.mcpCallAllowed(call, current) {
			return o, errors.New("host_not_authorized")
		}
		return s.mcp.operations.Observe(o.ID, state, result)
	}
	var stored struct {
		Task *mcpTaskReference `json:"task"`
	}
	if json.Unmarshal(o.Result, &stored) != nil || stored.Task == nil {
		return o, nil
	}
	task := stored.Task
	if !mcpTaskID(task.Owner, task.ID) {
		return o, errors.New("task_receipt_invalid")
	}
	path, query := "", ""
	switch task.Owner {
	case "backup":
		if op.Domain != "backups" {
			return o, errors.New("task_owner_invalid")
		}
		path = "/api/v1/backups/" + task.ID
	case "system":
		path = "/v1/system/management/config"
	case "partition":
		path = "/v1/system/disk-partitions"
	default:
		owner, ok := managedJobSources[task.Owner]
		if !ok || owner.Domain != op.Domain {
			return o, errors.New("task_owner_invalid")
		}
		path = owner.Path + "/" + task.ID
		if owner.Output {
			query = "projection=model-v1"
		}
		if task.Owner == "file_archive" {
			path, query = owner.Path, url.Values{"id": {task.ID}}.Encode()
		}
	}
	response, err := s.mcpLocalRequest(ctx, op, managedRequest{Method: "GET", Path: path, Query: query}, newRequestID())
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return o, errors.New("task_owner_unavailable")
	}
	var data map[string]any
	if json.Unmarshal(response.Body, &data) != nil {
		return o, errors.New("task_receipt_invalid")
	}
	if task.Owner == "system" {
		config, _ := data["state"].(map[string]any)
		data, _ = config["maintenance"].(map[string]any)
	} else if task.Owner == "partition" {
		data, _ = data["job"].(map[string]any)
	}
	// A newer maintenance task must never be mistaken for this operation.
	if data["id"] != task.ID {
		return o, errors.New("task_replaced_check_host_state")
	}
	if task.Owner == "file_archive" {
		if err := mcpArchiveScope(response.Body, call.principal.Policy); err != nil {
			return o, err
		}
	}
	body, _ := json.Marshal(data)
	output, err := mcpManagedOutput(body, 0, 20)
	if err != nil {
		return o, err
	}
	output["task"] = task
	result, _ := json.Marshal(output)
	if !s.mcpCallActive(call) {
		return o, errors.New("operation_not_authorized")
	}
	return s.mcp.operations.Observe(o.ID, mcpTaskState(data), result)
}

func mcpArchiveScope(body []byte, policy *mcpaccess.Policy) error {
	var job contract.FileArchiveJob
	if json.Unmarshal(body, &job) != nil || job.Target == "" || len(job.Sources) == 0 || !policy.AllowsPath(job.Target) || !aiFileMutable(job.Target) {
		return errors.New("file_root_not_authorized")
	}
	for _, path := range job.Sources {
		if !policy.AllowsPath(path) || !aiFileMutable(path) {
			return errors.New("file_root_not_authorized")
		}
	}
	return nil
}

func (s *Server) mcpPreflight(ctx context.Context, tool string, args json.RawMessage, policy *mcpaccess.Policy) error {
	if tool == "host_file_trash_action" {
		return s.mcpTrashPreflight(ctx, args, policy)
	}
	if tool != "host_file_archive_job" && tool != "host_file_archive_job_cancel" {
		return nil
	}
	var input managedJobQuery
	if json.Unmarshal(args, &input) != nil || !ownerJobIDPattern.MatchString(input.JobID) {
		return errors.New("invalid_job_id")
	}
	response, err := s.hostOps.Get(ctx, "/v1/files/archive-jobs", url.Values{"id": {input.JobID}}.Encode(), newRequestID())
	if err != nil || response.StatusCode != 200 {
		return errors.New("task_owner_unavailable")
	}
	return mcpArchiveScope(response.Body, policy)
}

// Raw terminal windows have lost the context needed for credential projection.
// Only the owner's explicitly versioned, already-projected display is accepted.
func mcpTerminalProjection(data map[string]any) (bool, error) {
	if _, exists := data["dataBase64"]; exists {
		return false, errors.New("terminal_projection_upgrade_required")
	}
	projection, exists := data["projection"]
	if !exists {
		return false, nil
	}
	if projection != "model-v1" {
		return false, errors.New("terminal_projection_invalid")
	}
	if _, exists := data["text"]; !exists {
		return false, nil
	}
	text, textOK := data["text"].(string)
	next, ok := data["nextOffset"].(float64)
	if projection != "model-v1" || !textOK || len(text) > 12<<10 || !ok || next < float64(len(text)) || next > 16<<20 || next != float64(int64(next)) {
		return false, errors.New("terminal_projection_invalid")
	}
	truncated, _ := data["truncated"].(bool)
	return truncated, nil
}
