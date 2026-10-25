package contract

import "testing"

func TestVMRequestRejectsInjectedAndIncompleteInputs(t *testing.T) {
	valid := VMRequest{Action: "create", RequestID: "11111111-1111-1111-1111-111111111111", Name: "test-vm", Source: "iso", Image: "install.iso", Pool: "default", Network: "default", CPUs: 2, MemoryMiB: 1024, DiskGiB: 20}
	if f, d := ValidateVMRequest(&valid); f != "" {
		t.Fatalf("valid request: %s %s", f, d)
	}
	for _, change := range []func(*VMRequest){func(r *VMRequest) { r.Name = "--connect" }, func(r *VMRequest) { r.Network = "default,script=/tmp/payload" }, func(r *VMRequest) { r.Pool = "../images" }, func(r *VMRequest) { r.Image = "../../etc/shadow.iso" }, func(r *VMRequest) { r.MemoryMiB = -1 }, func(r *VMRequest) { r.CPUs = 257 }, func(r *VMRequest) { r.DiskGiB = 0 }, func(r *VMRequest) { r.Source = "shell" }, func(r *VMRequest) { r.RequestID = "../../job" }, func(r *VMRequest) { r.DeleteDisks = true }} {
		r := valid
		change(&r)
		if f, _ := ValidateVMRequest(&r); f == "" {
			t.Fatalf("accepted invalid request: %#v", r)
		}
	}
}

func TestVMWritesRequireResourceVersion(t *testing.T) {
	r := VMRequest{Action: "delete", RequestID: "11111111-1111-1111-1111-111111111111", ID: "22222222-2222-2222-2222-222222222222"}
	if f, _ := ValidateVMRequest(&r); f == "" {
		t.Fatal("accepted missing version")
	}
	r.ExpectedResourceVersion = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if f, _ := ValidateVMRequest(&r); f != "" {
		t.Fatal(f)
	}
	r.Action = "exec"
	if f, _ := ValidateVMRequest(&r); f == "" {
		t.Fatal("accepted arbitrary action")
	}
}
