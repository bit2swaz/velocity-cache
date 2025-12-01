package local

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

func (d *LocalDriver) StartJanitor(retentionPeriod time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := d.cleanup(retentionPeriod); err != nil {
					log.Printf("Janitor error: %v", err)
				}
			}
		}
	}()
}

func (d *LocalDriver) cleanup(retention time.Duration) error {
	cutoff := time.Now().Add(-retention)

	return filepath.Walk(d.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return err
			}
			log.Printf("Janitor: Deleted expired cache %s", info.Name())
		}
		return nil
	})
}
