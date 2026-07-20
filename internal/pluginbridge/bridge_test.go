package pluginbridge

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBridgeLoadsCTFdStyleChatNotifier(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python not available")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	bridge, err := New(Config{
		Python:     python,
		Script:     filepath.Join(root, "python", "ctfd_bridge.py"),
		PluginDirs: []string{filepath.Join(root, "python", "ctfd_plugins")},
	})
	if err != nil {
		t.Fatalf("New bridge: %v", err)
	}
	defer bridge.Close()

	catalog := bridge.Catalog()
	if !contains(catalog.Notifiers, "chat_notifier") {
		t.Fatalf("chat_notifier not loaded: %+v", catalog)
	}
	if len(catalog.HomeWidgets) == 0 {
		t.Fatalf("expected home widget from CTFd-style load(app)")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
