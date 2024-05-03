package runtime

import (
	"fmt"
	"os"
	"testing"
)

func TestNewTempDir(t *testing.T) {
	tempDir, err := NewTempDir()
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(tempDir)

	dataDir, _ := DataDir()

	fmt.Println(tempDir, LogsDir(), dataDir)
}
