package telemetry_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

func TestBusPublishMetrics(t *testing.T) {
	bus := telemetry.NewBus("test-session", filepath.Join(t.TempDir(), "missing.sock"))
	bus.Publish(telemetry.Event{Type: telemetry.EventEchoTrip})
	m := bus.Metrics()
	if m.EchoTrips != 1 {
		t.Fatalf("echo trips: %d", m.EchoTrips)
	}
}

func TestControlResume(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "control.sock")
	called := false
	srv := telemetry.NewControlServer(socket, "sess-1", func(_ string) { called = true })
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	if err := telemetry.SendResume(socket, "sess-1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if !called {
		t.Fatal("resume callback not invoked")
	}
}

func TestEventServerAccept(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "events.sock")
	ln, err := telemetry.StartEventServer(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	defer os.Remove(sock)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
}
