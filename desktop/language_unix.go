//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func systemLanguages() []string {
	var values []string
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
			values = strings.FieldsFunc(string(output), func(r rune) bool {
				return strings.ContainsRune("(),\" \n\r\t", r)
			})
		}
	}
	values = append(values, strings.Split(os.Getenv("LANGUAGE"), ":")...)
	return append(values, os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG"))
}
