//go:build windows

package windowsnode

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var lifecycleTestSequence atomic.Uint64

func lifecycleTestName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`Local\KPanelLifecycle-%d-%d-%d`, os.Getpid(), time.Now().UnixNano(), lifecycleTestSequence.Add(1))
}

func lifecycleTestObject(t *testing.T, name, descriptor string) windows.Handle {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateMutex(&sa, false, ptr)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		t.Fatal(err)
	}
	return h
}

func TestLifecycleRejectsUntrustedPreexistingMutex(t *testing.T) {
	for _, descriptor := range []string{
		"D:P(A;;GA;;;WD)",
		"D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x00100001;;;AU)",
		"D:NO_ACCESS_CONTROL",
	} {
		t.Run(descriptor, func(t *testing.T) {
			name := lifecycleTestName(t)
			h := lifecycleTestObject(t, name, descriptor)
			defer windows.CloseHandle(h)
			if release, err := acquireLifecycleMutex(name); err == nil {
				_ = release()
				t.Fatal("lifecycle accepted an object granting untrusted access")
			}
		})
	}
}

// The old protocol creates/opens the public mutex and waits on it without
// checking its owner or DACL. Exercise that exact behavior on the test thread.
func TestLifecycleCoordinatesWithLegacyBothDirections(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := lifecycleTestName(t)
	legacy := lifecycleTestObject(t, name, "D:P(A;;GA;;;SY)(A;;GA;;;BA)")
	defer windows.CloseHandle(legacy)
	if result, err := windows.WaitForSingleObject(legacy, 0); err != nil || result != windows.WAIT_OBJECT_0 {
		t.Fatalf("legacy acquire: result=%d error=%v", result, err)
	}
	if release, err := acquireLifecycleMutex(name); err == nil {
		_ = release()
		_ = windows.ReleaseMutex(legacy)
		t.Fatal("new operation overlapped legacy owner")
	}
	if err := windows.ReleaseMutex(legacy); err != nil {
		t.Fatal(err)
	}
	release, err := acquireLifecycleMutex(name)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if result, err := windows.WaitForSingleObject(legacy, 0); err != nil || result != uint32(windows.WAIT_TIMEOUT) {
		if result == windows.WAIT_OBJECT_0 {
			_ = windows.ReleaseMutex(legacy)
		}
		t.Fatalf("legacy bypassed new owner: result=%d error=%v", result, err)
	}
}

func TestLifecycleDoesNotOverlapLegacyOnUntrustedObject(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := lifecycleTestName(t)
	legacy := lifecycleTestObject(t, name, "D:P(A;;GA;;;WD)")
	defer windows.CloseHandle(legacy)
	if result, err := windows.WaitForSingleObject(legacy, 0); err != nil || result != windows.WAIT_OBJECT_0 {
		t.Fatalf("legacy acquire: result=%d error=%v", result, err)
	}
	defer windows.ReleaseMutex(legacy)
	if release, err := acquireLifecycleMutex(name); err == nil {
		_ = release()
		t.Fatal("new operation overlapped legacy operation on an untrusted object")
	}
}

// Restrict only this thread's token. No account, service or persistent object
// is installed, and impersonation is reverted before returning to the scheduler.
func withLifecycleRestrictedToken(t *testing.T, fn func()) {
	t.Helper()
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	disabled := windows.SIDAndAttributes{Sid: admin}
	var primary windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &primary); err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	var restricted windows.Token
	create := windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")
	ok, _, callErr := create.Call(uintptr(primary), 1, 1, uintptr(unsafe.Pointer(&disabled)), 0, 0, 0, 0, uintptr(unsafe.Pointer(&restricted)))
	runtime.KeepAlive(admin)
	if ok == 0 {
		t.Fatalf("create restricted token: %v", callErr)
	}
	defer restricted.Close()
	var token windows.Token
	if err := windows.DuplicateTokenEx(restricted, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE, nil,
		windows.SecurityImpersonation, windows.TokenImpersonation, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	if member, err := token.IsMember(admin); err != nil || member {
		t.Fatalf("restricted token has enabled administrators SID: %v %v", member, err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.SetThreadToken(nil, token); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := windows.RevertToSelf(); err != nil {
			panic(err)
		}
	}()
	fn()
}

func TestLifecycleRestrictedTokenCannotOpenTrustedMutex(t *testing.T) {
	name := lifecycleTestName(t)
	release, err := acquireLifecycleMutex(name)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	withLifecycleRestrictedToken(t, func() {
		ptr, _ := windows.UTF16PtrFromString(name)
		h, err := windows.OpenMutex(windows.MUTEX_MODIFY_STATE|windows.SYNCHRONIZE, false, ptr)
		if err == nil {
			windows.CloseHandle(h)
			t.Fatal("restricted token opened the trusted mutex")
		}
	})
}

func TestLifecycleRejectsRestrictedOwnerAndRecoversAfterClose(t *testing.T) {
	name := lifecycleTestName(t)
	var squatter windows.Handle
	withLifecycleRestrictedToken(t, func() {
		squatter = lifecycleTestObject(t, name, "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;OW)")
		// A restrictive replacement DACL cannot revoke the creator's existing
		// full-access handle. Owner validation must still reject the object.
		sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)")
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := sd.DACL()
		if err != nil {
			t.Fatal(err)
		}
		if err := windows.SetSecurityInfo(squatter, windows.SE_KERNEL_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			t.Fatal(err)
		}
	})
	if release, err := acquireLifecycleMutex(name); err == nil {
		_ = release()
		t.Fatal("accepted a restricted owner's object")
	}
	// Closing the final handle destroys the object and permits a trusted retry.
	if err := windows.CloseHandle(squatter); err != nil {
		t.Fatal(err)
	}
	release, err := acquireLifecycleMutex(name)
	if err != nil {
		t.Fatalf("retry after squatter exit: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleRejectsWrongTypeAndDeniedObject(t *testing.T) {
	name := lifecycleTestName(t)
	ptr, _ := windows.UTF16PtrFromString(name)
	event, err := windows.CreateEvent(nil, 0, 0, ptr)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(event)
	if release, err := acquireLifecycleMutex(name); err == nil {
		_ = release()
		t.Fatal("accepted a same-name event")
	}
	name = lifecycleTestName(t)
	denied := lifecycleTestObject(t, name, "D:P(D;;GA;;;WD)(A;;GA;;;SY)(A;;GA;;;BA)")
	defer windows.CloseHandle(denied)
	if release, err := acquireLifecycleMutex(name); err == nil {
		_ = release()
		t.Fatal("bypassed an inaccessible object")
	}
}

func TestLifecycleLegacyProcessAndAbandonedOwner(t *testing.T) {
	for _, mode := range []string{"hold", "abandon", "probe"} {
		t.Run(mode, func(t *testing.T) {
			name := fmt.Sprintf(`Global\KPanelLifecycleProcess-%d-%d`, os.Getpid(), time.Now().UnixNano())
			if mode == "probe" {
				release, err := acquireLifecycleMutex(name)
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleLegacyProcessHelper$")
			child.Env = append(os.Environ(), "KPANEL_TEST_LEGACY_MUTEX="+name, "KPANEL_TEST_LEGACY_MODE="+mode)
			var diagnostics bytes.Buffer
			child.Stderr = &diagnostics
			input, err := child.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			output, err := child.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(output)
			message, err := reader.ReadString('\n')
			if err != nil || message != "ready\n" {
				_ = input.Close()
				_ = child.Wait()
				t.Fatalf("legacy child: %q %v %s", message, err, diagnostics.String())
			}
			if mode != "probe" {
				if release, err := acquireLifecycleMutex(name); err == nil {
					_ = release()
					t.Error("new operation overlapped legacy process")
				}
			}
			// Keep the object alive across child exit so WAIT_ABANDONED is
			// actually exercised, rather than creating a new empty mutex.
			ptr, _ := windows.UTF16PtrFromString(name)
			retained, err := windows.OpenMutex(windows.SYNCHRONIZE, false, ptr)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(retained)
			_ = input.Close()
			if err := child.Wait(); err != nil {
				t.Fatalf("legacy child exit: %v %s", err, diagnostics.String())
			}
			if mode != "probe" {
				release, err := acquireLifecycleMutex(name)
				if err != nil {
					t.Fatalf("acquire after legacy exit: %v", err)
				}
				if err := release(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestLifecycleLegacyProcessHelper(t *testing.T) {
	name, mode := os.Getenv("KPANEL_TEST_LEGACY_MUTEX"), os.Getenv("KPANEL_TEST_LEGACY_MODE")
	if name == "" {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	h := lifecycleTestObject(t, name, "D:P(A;;GA;;;SY)(A;;GA;;;BA)")
	defer windows.CloseHandle(h)
	result, err := windows.WaitForSingleObject(h, 0)
	if mode == "probe" {
		if err != nil || result != uint32(windows.WAIT_TIMEOUT) {
			t.Fatalf("legacy process bypassed new owner: %d %v", result, err)
		}
	} else if err != nil || result != windows.WAIT_OBJECT_0 {
		t.Fatalf("legacy process acquire: %d %v", result, err)
	}
	fmt.Fprintln(os.Stdout, "ready")
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
	if mode == "abandon" {
		os.Exit(0)
	}
	if mode != "probe" {
		if err := windows.ReleaseMutex(h); err != nil {
			t.Fatal(err)
		}
	}
}
