// logger/logger.go
package logger

import (
	"log"
	"runtime"
)

func LogWithFuncName(message string) {
	pc, _, _, _ := runtime.Caller(1) // Get the caller's information
	funcName := runtime.FuncForPC(pc).Name()
	log.Printf("[%s]: %s", funcName, message)
}
