package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/store"
)

// Measure production password parameters separately from the persistence path.
// The fast-hasher case isolates storage overhead; it is never used by paneld.
func BenchmarkLogin(b *testing.B) {
	for _, production := range []bool{true, false} {
		name := "ProductionArgon2id"
		var hasher PasswordHasher = benchmarkPasswordHasher{}
		if production {
			var err error
			hasher, err = NewArgon2idHasher(DefaultArgon2idParams())
			if err != nil {
				b.Fatal(err)
			}
		} else {
			name = "PersistenceOnly"
		}
		b.Run(name, func(b *testing.B) {
			directory := b.TempDir()
			storage, err := store.Open(filepath.Join(directory, "state.json"))
			if err != nil {
				b.Fatal(err)
			}
			defer storage.Close()
			service, err := NewService(storage, hasher, Config{BootstrapTokenPath: filepath.Join(directory, "bootstrap.token"), SessionTTL: time.Hour})
			if err != nil {
				b.Fatal(err)
			}
			if err := service.EnsureBootstrapToken(); err != nil {
				b.Fatal(err)
			}
			token, err := os.ReadFile(service.config.BootstrapTokenPath)
			if err != nil {
				b.Fatal(err)
			}
			credentials, err := service.Bootstrap(string(token), "admin", "benchmark-password-123")
			if err != nil {
				b.Fatal(err)
			}
			if err := service.Logout(credentials.Token); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				credentials, err := service.Login("192.0.2.1", "admin", "benchmark-password-123", "")
				if err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				if err := service.Logout(credentials.Token); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
			}
		})
	}
}

type benchmarkPasswordHasher struct{}

func (benchmarkPasswordHasher) Hash(password string) (string, error) { return password, nil }
func (benchmarkPasswordHasher) Verify(password, encoded string) (bool, error) {
	return password == encoded, nil
}
