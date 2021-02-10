package log

import (
	l "log"
)

var debugEnabled bool = false

func EnableDebug() {
	debugEnabled = true
}

func Printf(format string, args ...interface{}) {
	l.Printf(format, args...)
}

func Println(args ...interface{}) {
	l.Println(args...)
}

func Debugf(format string, args ...interface{}) {
	if debugEnabled {
		l.Printf(format, args...)
	}
}

func Debugln(args ...interface{}) {
	if debugEnabled {
		l.Println(args...)
	}
}
