package runtime

import (
	"os"
	"path/filepath"
)

func DataDir() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		// 默认在 ~/.local/share
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
