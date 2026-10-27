package cluster

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBatchAttemptRevokedWhenHostIsDeleted(t *testing.T) {
	for _, platform := range []string{"linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			service := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
			batch, err := service.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{MaxUses: 2})
			if err != nil {
				t.Fatal(err)
			}
			input := LightEnrollRequest{Platform: platform, Token: lightBatchTokenForTest(t, batch), Name: "edge", AttemptID: strings.Repeat("a", 32)}
			first, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input)
			if err != nil {
				t.Fatal(err)
			}
			retry, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input)
			if err != nil || retry.NodeID != first.NodeID || retry.ReportingKey != first.ReportingKey {
				t.Fatalf("valid retry=%#v err=%v", retry, err)
			}
			record, err := service.light.Host(first.NodeID)
			if err != nil {
				t.Fatal(err)
			}
			now := service.now().UTC()
			body, auth := signedLightReportForTest(t, now, first, LightReportRequest{Telemetry: serviceTelemetry(now, "fixture")}, strings.Repeat("d", 32))
			if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: "sha256:" + strings.Repeat("0", 64)}); !errors.Is(err, ErrConflict) {
				t.Fatalf("stale delete=%v", err)
			}
			if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); err != nil {
				t.Fatalf("stale delete revoked attempt: %v", err)
			}
			if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := service.authenticateLightRequest(auth, lightReportPath, body, 1<<20, now); !errors.Is(err, ErrAuthentication) {
				t.Fatalf("deleted reporting key authenticated: %v", err)
			}
			reopened, err := openLightBatchStore(service.lightBatches.path)
			if err != nil {
				t.Fatal(err)
			}
			service.lightBatches = reopened
			if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); !errors.Is(err, ErrPairingCode) {
				t.Fatalf("deleted attempt reentered: %v", err)
			}
			if got := service.LightBatchEnrollments().Items[0]; got.UsedCount != 1 || got.RemainingCount != 1 {
				t.Fatalf("deleted attempt changed quota: %#v", got)
			}
			input.AttemptID = strings.Repeat("b", 32)
			replacement, _, err := service.EnrollLightNodeBatch("198.51.100.11", service.publicURL, input)
			if err != nil || replacement.NodeID == first.NodeID {
				t.Fatalf("new attempt=%#v err=%v", replacement, err)
			}
		})
	}
}

func TestBatchReservedAttemptCanRetryAfterHostCreationFails(t *testing.T) {
	service := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	batch, err := service.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{MaxUses: 1})
	if err != nil {
		t.Fatal(err)
	}
	input := LightEnrollRequest{Token: lightBatchTokenForTest(t, batch), AttemptID: strings.Repeat("e", 32)}
	oldOps := service.light.ops
	service.light.ops.syncFile = func(*os.File) error { return errors.New("injected host creation failure") }
	if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); err == nil {
		t.Fatal("expected failed host creation")
	}
	service.light.ops = oldOps
	if got := service.LightBatchEnrollments().Items[0]; got.RemainingCount != 0 {
		t.Fatalf("reservation was not retained: %#v", got)
	}
	if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); err != nil {
		t.Fatalf("reserved attempt failed to recover: %v", err)
	}
}

func TestBatchRemovalPersistenceFailuresFailClosed(t *testing.T) {
	service := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	batch, err := service.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{MaxUses: 1})
	if err != nil {
		t.Fatal(err)
	}
	input := LightEnrollRequest{Token: lightBatchTokenForTest(t, batch), AttemptID: strings.Repeat("c", 32)}
	first, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input)
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.light.Host(first.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	oldBatchOps := service.lightBatches.ops
	service.lightBatches.ops.syncFile = func(*os.File) error { return errors.New("injected batch write failure") }
	if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err == nil {
		t.Fatal("delete succeeded despite failed revocation")
	}
	service.lightBatches.ops = oldBatchOps
	if _, err := service.light.Host(first.NodeID); err != nil {
		t.Fatalf("failed revocation removed host: %v", err)
	}
	if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); err != nil {
		t.Fatalf("failed revocation changed retry: %v", err)
	}
	oldHostOps := service.light.ops
	service.light.ops.syncFile = func(*os.File) error { return errors.New("injected host write failure") }
	if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err == nil {
		t.Fatal("delete succeeded despite failed host write")
	}
	service.light.ops = oldHostOps
	if _, err := service.light.Host(first.NodeID); err != nil {
		t.Fatalf("failed host write removed host: %v", err)
	}
	if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); !errors.Is(err, ErrPairingCode) {
		t.Fatalf("durably revoked attempt reentered: %v", err)
	}
	if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err != nil {
		t.Fatalf("retry delete failed: %v", err)
	}
}

func TestBatchRemovalRequiresDurableBackupCleanup(t *testing.T) {
	service := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	batch, err := service.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{MaxUses: 1})
	if err != nil {
		t.Fatal(err)
	}
	input := LightEnrollRequest{Token: lightBatchTokenForTest(t, batch), AttemptID: strings.Repeat("f", 32)}
	first, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input)
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.light.Host(first.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	oldOps := service.lightBatches.ops
	service.lightBatches.ops.remove = func(path string) error {
		if path == service.lightBatches.path+".previous" {
			return errors.New("injected backup removal failure")
		}
		return oldOps.remove(path)
	}
	if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err == nil {
		t.Fatal("delete succeeded while old backup remained")
	}
	if _, err := service.light.Host(first.NodeID); err != nil {
		t.Fatalf("host removed despite failed cleanup: %v", err)
	}
	if _, _, err := service.EnrollLightNodeBatch("198.51.100.10", service.publicURL, input); !errors.Is(err, ErrPairingCode) {
		t.Fatalf("persisted revocation was undone: %v", err)
	}
	service.lightBatches.ops = oldOps
	if _, err := service.DeleteHost(context.Background(), first.NodeID, DeleteHostInput{ExpectedResourceVersion: record.ResourceVersion}); err != nil {
		t.Fatalf("delete retry failed: %v", err)
	}
	if _, err := os.Lstat(service.lightBatches.path + ".previous"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old backup remains: %v", err)
	}
}
