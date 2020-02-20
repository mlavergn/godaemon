// +build linux

package daemon

import (
	"io/ioutil"
	"strconv"
	"strings"
)

// ProcessInfoNameImpl export
func ProcessInfoNameImpl(pid int) string {
	statPath := "/proc/" + strconv.Itoa(pid) + "/stat"
	data, err := ioutil.ReadFile(statPath)
	if err != nil {
		log.Println("Failed to read process name", pid, err)
		return ""
	}
	// executable name is first value in parens
	value := string(data)
	start := strings.IndexRune(value, '(') + 1
	end := strings.IndexRune(value[start:], ')')

	return value[start : start+end]
}
