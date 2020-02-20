// +build darwin

package daemon

// #include <libproc.h>
// #include <stdlib.h>
import "C"

// ProcessInfoNameImpl export
func ProcessInfoNameImpl(pid int) string {
	cname := C.malloc(C.PROC_PIDPATHINFO_MAXSIZE)
	defer C.free(cname)
	_, err := C.proc_pidpath(C.int(pid), cname, C.PROC_PIDPATHINFO_MAXSIZE)
	if err != nil {
		log.Println("Failed to read process name", pid, err)
		return ""
	}
	return C.GoString((*C.char)(cname))
}
