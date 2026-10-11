package contract

import (
	"strings"
	"testing"
)

func TestSystemServiceNamesAndActionsRejectInjection(t *testing.T) {
	for _, name := range []string{"ssh.service", "worker@blue.service", `path\x2dworker.service`, "kejilion-agent.service"} {
		if !ValidSystemServiceName(name) {
			t.Errorf("valid name rejected: %q", name)
		}
	}
	for _, name := range []string{"", ".service", "ssh", "ssh.socket", "--help.service", "*.service", "a?.service", "a[0].service", "../ssh.service", "/etc/ssh.service", "ssh.service\n", "ssh;id.service", "$(id).service", "ssh service", `bad\oops.service`, "a%20.service", strings.Repeat("a", 248) + ".service"} {
		if ValidSystemServiceName(name) {
			t.Errorf("unsafe name accepted: %q", name)
		}
	}
	for _, action := range []string{"start", "stop", "restart", "reload", "enable", "disable"} {
		input := SystemServiceActionRequest{Name: "ssh.service", Action: action, ExpectedResourceVersion: strings.Repeat("a", 64)}
		if field, _ := ValidateSystemServiceAction(&input); field != "" {
			t.Errorf("valid %s: %s", action, field)
		}
	}
	for _, input := range []SystemServiceActionRequest{
		{Name: "ssh.service", Action: "mask", ExpectedResourceVersion: strings.Repeat("a", 64)},
		{Name: "ssh.service", Action: "start", ExpectedResourceVersion: "old"},
		{Name: "*.service", Action: "stop", ExpectedResourceVersion: strings.Repeat("a", 64)},
	} {
		if field, _ := ValidateSystemServiceAction(&input); field == "" {
			t.Errorf("invalid input accepted: %#v", input)
		}
	}
}

func TestSystemServicePathsAreSingleExactSegments(t *testing.T) {
	for _, path := range []string{"/system/services/ssh.service", "/system/services/worker@a.service/logs"} {
		if _, _, ok := ParseSystemServicePath(path, "/system/services"); !ok {
			t.Errorf("valid path rejected %s", path)
		}
	}
	for _, path := range []string{"/system/services", "/system/services/../ssh.service", "/system/services/ssh.service/", "/system/services/ssh.service/logs/all", "/system/services/ssh.service/other.service", "/system/services/a%2f.service"} {
		if _, _, ok := ParseSystemServicePath(path, "/system/services"); ok {
			t.Errorf("invalid path accepted %s", path)
		}
	}
}
