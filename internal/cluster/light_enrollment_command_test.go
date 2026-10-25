package cluster

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLightEnrollmentCommandKeepsAuthorizationOutOfInstallerArguments(t *testing.T) {
	service := &Service{}
	for _, token := range []string{"kpl1.synthetic_authorization", "kpb1.synthetic_authorization"} {
		command, err := service.lightEnrollmentCommand(token, "node 'one'")
		if err != nil {
			t.Fatal(err)
		}
		arguments, input, found := strings.Cut(command, " 3<<< ")
		if !found || strings.Contains(arguments, token) || input != shellSingleQuote(token) {
			t.Fatalf("authorization must only occur in protected descriptor input: %q", command)
		}
		if !strings.Contains(arguments, "--name "+shellSingleQuote("node 'one'")) {
			t.Fatalf("display name lost quoting: %q", command)
		}
	}
}

func TestLocalEnrollmentReferenceCannotAuthorizeAtCenter(t *testing.T) {
	service := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	for _, batch := range []bool{false, true} {
		var command string
		if batch {
			enrollment, err := service.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{})
			if err != nil {
				t.Fatal(err)
			}
			command = enrollment.Command
		} else {
			enrollment, err := service.CreateLightEnrollment()
			if err != nil {
				t.Fatal(err)
			}
			command = enrollment.Command
		}
		arguments, _, _ := strings.Cut(command, " 3<<< ")
		fields := strings.Fields(arguments)
		reference := strings.Trim(fields[len(fields)-1], "'")
		_, body, _ := strings.Cut(reference, ".")
		content, err := base64.RawURLEncoding.DecodeString(body)
		if err != nil {
			t.Fatal(err)
		}
		var decoded LightEnrollmentTokenReference
		if err := json.Unmarshal(content, &decoded); err != nil || decoded.Version != 2 || decoded.TokenFD != 3 {
			t.Fatalf("invalid local reference: %#v, %v", decoded, err)
		}
		if batch {
			if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", "https://panel.example", LightEnrollRequest{Token: reference, Name: "node", NodeVersion: "0.40.0", AttemptID: strings.Repeat("a", 32)}); err == nil {
				t.Fatal("batch center accepted a local FD reference")
			}
		} else if _, err := service.EnrollLightNode("198.51.100.10", LightEnrollRequest{Token: reference, Name: "node", NodeVersion: "0.40.0"}); err == nil {
			t.Fatal("center accepted a local FD reference")
		}
	}
}
