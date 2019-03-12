// +build linux,!arm

package main

import "fmt"

type kuUserPrint struct {
}

func newKuPrint() (*kuUserPrint, error) {
	kup := &kuUserPrint{}
	return kup, nil
}

func (kup *kuUserPrint) kuPrintln(a ...interface{}) (n int, err error) {
	fmt.Println(a...)
}

func (kup *kuUserPrint) kuClose() {
}
