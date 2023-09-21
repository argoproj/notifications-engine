package docs

import (
	"os"
	"path/filepath"

	"github.com/argoproj/notifications-engine/docs/services"
)

// CopyServicesDocs copies markdown files with the services docs to the target directory and returns list of copied files
func CopyServicesDocs(dest string) ([]string, error) {
	entries, err := services.Docs.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		destName := filepath.Join(dest, entry.Name())
		destDir := filepath.Dir(entry.Name())
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, err
		}

		data, err := services.Docs.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(destName, data, 0755)
		if err != nil {
			return nil, err
		}
		names = append(names, destName)
	}
	return names, nil
}
