package runtime

import (
	"os"
	"path/filepath"
)

func DataDir() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		// 默认在 ~/.single/share
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	regDir := filepath.Join(dataDir, "serviceakasar")
	if err := os.MkdirAll(regDir, 0700); err != nil {
		return "", err
	}

	return regDir, nil
}

func NewTempDir() (string, error) {
	tmpDir, err := os.MkdirTemp("", "akasar")
	if err != nil {
		return "", err
	}

	if err := os.Chmod(tmpDir, 0700); err != nil {
		_ = os.Remove(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

func LogsDir() string {
	return filepath.Join(os.TempDir(), "serviceakasar", "logs")
}
