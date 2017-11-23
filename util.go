package main

import (
	"os"
	"path/filepath"
	"runtime"
)

//getCPUNum get cpu number
func getCPUNum() int {
	return runtime.NumCPU()
}

//prefixWork prefixWork
func (r *Run) prefixWork() error {
	workDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	r.screenDir = filepath.Join(workDir, pngDir)
	if _, err := os.Stat(r.screenDir); os.IsNotExist(err) {
		return os.Mkdir(r.screenDir, 0774)
	}

	r.seleniumPath = filepath.Join(workDir, "vendor", seleniumFile)
	if _, err := os.Stat(r.seleniumPath); os.IsNotExist(err) {
		return err
	}
	return nil
}
