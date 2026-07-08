package store

import (
	"path/filepath"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleClient(name string) model.Client {
	return model.Client{
		Name:              name,
		UUID:              "uuid-" + name,
		Password:          "pw-" + name,
		SS2022Key:         "ss-" + name,
		ShadowTLSPassword: "st-" + name,
		SubToken:          "tok-" + name,
		Enabled:           true,
		QuotaBytes:        0,
		CreatedAt:         1000,
	}
}

func TestCreateAndList(t *testing.T) {
	s := openTemp(t)
	a, err := s.CreateClient(sampleClient("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 {
		t.Fatal("expected assigned id")
	}
	if _, err := s.CreateClient(sampleClient("bob")); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListClients()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(list))
	}
	if list[0].Name != "alice" || list[1].Name != "bob" {
		t.Errorf("unexpected order/names: %+v", list)
	}
}

func TestUniqueNameRejected(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateClient(sampleClient("dup")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClient(sampleClient("dup")); err == nil {
		t.Fatal("expected unique-constraint error on duplicate name")
	}
}

func TestGetByTokenAndEnabledFilter(t *testing.T) {
	s := openTemp(t)
	a, _ := s.CreateClient(sampleClient("alice"))
	b := sampleClient("bob")
	b.Enabled = false
	if _, err := s.CreateClient(b); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetClientByToken("tok-alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != a.ID {
		t.Errorf("token lookup returned wrong client")
	}

	active, err := s.ActiveClients(2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Name != "alice" {
		t.Errorf("ActiveClients should exclude disabled bob: %+v", active)
	}
}

func TestActiveClientsExcludesExpired(t *testing.T) {
	s := openTemp(t)
	live := sampleClient("live")
	live.ExpiresAt = 5000
	if _, err := s.CreateClient(live); err != nil {
		t.Fatal(err)
	}
	expired := sampleClient("expired")
	expired.ExpiresAt = 1000
	if _, err := s.CreateClient(expired); err != nil {
		t.Fatal(err)
	}

	// At now=3000: live (exp 5000) active, expired (exp 1000) not.
	active, err := s.ActiveClients(3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Name != "live" {
		t.Errorf("ActiveClients should exclude expired: %+v", active)
	}
}

func TestDeviceLimitAndExpiryRoundTrip(t *testing.T) {
	s := openTemp(t)
	c := sampleClient("carol")
	c.DeviceLimit = 3
	c.ExpiresAt = 1893456000
	created, err := s.CreateClient(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetClient(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceLimit != 3 || got.ExpiresAt != 1893456000 {
		t.Errorf("device/expiry not persisted: %+v", got)
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetClient(999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTrafficAccumulateAndReset(t *testing.T) {
	s := openTemp(t)
	c, _ := s.CreateClient(sampleClient("alice"))

	if err := s.AddTraffic(c.ID, 100, 200, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTraffic(c.ID, 50, 25, 6); err != nil {
		t.Fatal(err)
	}
	tr, err := s.GetTraffic(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Up != 150 || tr.Down != 225 || tr.UpdatedAt != 6 {
		t.Errorf("traffic = %+v, want up=150 down=225 at=6", tr)
	}

	if err := s.ResetTraffic(c.ID); err != nil {
		t.Fatal(err)
	}
	tr, _ = s.GetTraffic(c.ID)
	if tr.Up != 0 || tr.Down != 0 {
		t.Errorf("traffic after reset = %+v, want zero", tr)
	}
}

func TestDeleteRemovesClientAndTraffic(t *testing.T) {
	s := openTemp(t)
	c, _ := s.CreateClient(sampleClient("alice"))
	if err := s.DeleteClient(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetClient(c.ID); err != ErrNotFound {
		t.Fatalf("client should be gone, got %v", err)
	}
}

func TestSetEnabledAndQuota(t *testing.T) {
	s := openTemp(t)
	c, _ := s.CreateClient(sampleClient("alice"))
	if err := s.SetEnabled(c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetQuota(c.ID, 1<<30); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetClient(c.ID)
	if got.Enabled {
		t.Error("client should be disabled")
	}
	if got.QuotaBytes != 1<<30 {
		t.Errorf("quota = %d, want %d", got.QuotaBytes, int64(1<<30))
	}
}
