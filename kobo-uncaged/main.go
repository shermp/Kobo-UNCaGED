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
	"log"
	"log/syslog"
	"os"

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
	w, err := syslog.New(syslog.LOG_DEBUG, "KoboUNCaGED")
	if err == nil {
		log.SetOutput(w)
	}
	onboardMntPtr := flag.String("onboardmount", "/mnt/onboard", "If changed, specify the new new mountpoint of '/mnt/onboard'")
	sdMntPtr := flag.String("sdmount", "", "If changed, specify the new new mountpoint of '/mnt/sd'")
	bindAddrPtr := flag.String("bindaddr", "127.0.0.1:8181", "Specify the network address and port <IP:POrt> to listen on")

	flag.Parse()
	log.Println("Started Kobo-UNCaGED")
	log.Println("Reading options")
	log.Println("Creating KU object")
	k, err := device.New(*onboardMntPtr, *sdMntPtr, *bindAddrPtr, kuVersion)
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
	if len(k.UpdatedMetadata) > 0 {
		if err := k.WriteUpdatedMetadataSQL(); err != nil {
			k.FinishedMsg = "Updating metadata failed"
			log.Print(err)
			return returncodeFromError(err, k)
		}
		k.FinishedMsg = "Calibre disconnected<br>Metadata will be updated"
	}
	if k.BrowserOpen {
		k.FinishedMsg = "Calibre disconnected<br><br>You may exit the browser."
	} else {
		k.FinishedMsg = "Calibre disconnected"
	}
	return succsess
}
func main() {
	os.Exit(int(mainWithErrCode()))
}
