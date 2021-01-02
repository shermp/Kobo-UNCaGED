// Copyright 2019-2020 Sherman Perry

// This file is part of Kobo UNCaGED.

// Kobo UNCaGED is free software: you can redistribute it and/or modify
// it under the terms of the Affero GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Kobo UNCaGED is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with Kobo UNCaGED.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"log/syslog"
	"os"

	"net/http"
	_ "net/http/pprof"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/device"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/kunc"
	"github.com/shermp/UNCaGED/uc"
)

type returnCode int

// Note, this is set by the go linker at build time
var kuVersion string

const (
	genericError     returnCode = 250
	succsess         returnCode = 0
	successEarlyExit returnCode = 10
	passwordError    returnCode = 100
	calibreNotFound  returnCode = 101
)

func returncodeFromError(err error, k *device.Kobo) returnCode {
	rc := succsess
	if err != nil {
		log.Print(err)
		if k == nil {
			return genericError
		}
		k.FinishedMsg = err.Error()
		rc = genericError
		var calErr uc.CalError
		if errors.As(err, &calErr) {
			switch calErr {
			case uc.CalibreNotFound:
				k.FinishedMsg = "Calibre not found!<br>Have you enabled the Calibre Wireless service?"
				rc = calibreNotFound
			case uc.NoPassword:
				k.FinishedMsg = "No valid password found!"
				rc = passwordError
			default:
				k.FinishedMsg = calErr.Error()
				rc = genericError
			}
		}
	}
	return rc
}
func mainWithErrCode() returnCode {
	kuLogFile := os.Getenv("KU_LOGFILE")
	var w io.Writer
	var err error
	log.SetPrefix("[Kobo-UNCaGED] ")
	if kuLogFile != "" {
		w, err = os.OpenFile(kuLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		w, err = syslog.New(syslog.LOG_DEBUG, "Kobo-UNCaGED")
		log.SetPrefix("")
	}
	if err == nil {
		log.SetOutput(w)
	}

	onboardMntPtr := flag.String("onboardmount", "/mnt/onboard", "If changed, specify the new new mountpoint of '/mnt/onboard'")
	sdMntPtr := flag.String("sdmount", "", "If changed, specify the new new mountpoint of '/mnt/sd'")
	bindAddrPtr := flag.String("bindaddr", "127.0.0.1:8181", "Specify the network address and port <IP:POrt> to listen on")
	disableNDBPtr := flag.Bool("disablendb", false, "Disables use of NickelDBus. Useful for desktop testing")

	flag.Parse()
	log.Println("Started Kobo-UNCaGED")
	log.Println("Creating KU object")
	k, err := device.New(*onboardMntPtr, *sdMntPtr, *bindAddrPtr, *disableNDBPtr, kuVersion)
	if err != nil {
		log.Print(err)
		return returncodeFromError(err, nil)
	} else if k == nil {
		return successEarlyExit // the user exited during config
	}
	defer k.Close()

	log.Println("Preparing Kobo UNCaGED!")
	ku := kunc.New(k)
	cc, err := uc.New(ku, k.KuConfig.EnableDebug)
	if err != nil {
		log.Print(err)
		return returncodeFromError(err, k)
	}
	log.Println("Starting Calibre Connection")
	err = cc.Start()
	if err != nil {
		log.Print(err)
		return returncodeFromError(err, k)
	}
	if err = k.WritePassCache(); err != nil {
		// Not fatal, just log it
		log.Print(err)
	}
	if err = k.SaveUserOptions(); err != nil {
		// Annoying, but not fatal
		log.Print(err)
	}
	updateReq, err := k.WriteUpdatedMetadataSQL()
	if err != nil {
		k.FinishedMsg = "Updating metadata failed"
		log.Print(err)
		return returncodeFromError(err, k)
	}
	if k.BrowserOpen {
		if updateReq {
			k.FinishedMsg = "Calibre disconnected<br>Metadata will be updated<br><br>Please wait"
		} else {
			k.FinishedMsg = "Calibre disconnected<br><br>Please wait"
		}
	} else {
		if updateReq {
			k.FinishedMsg = "Calibre disconnected\nMetadata will be updated\n\nPlease wait"
		} else {
			k.FinishedMsg = "Calibre disconnected\n\nPlease wait"
		}
	}
	return succsess
}
func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	os.Exit(int(mainWithErrCode()))
}
