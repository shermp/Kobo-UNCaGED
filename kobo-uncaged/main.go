// Copyright 2019 Sherman Perry

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
	"flag"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/device"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/kunc"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/kuprint"
	"github.com/shermp/UNCaGED/uc"
	"github.com/pkg/errors"
)

type returnCode int

// Note, this is set by the go linker at build time
var kuVersion string

const (
	kuError           returnCode = 250
	kuSuccessNoAction returnCode = 0
	kuSuccessRerun    returnCode = 1
	kuPasswordError   returnCode = 100
)

func getUserOptions(dbRootDir string) (*device.KuOptions, error) {
	// Note, we return opts, regardless of whether we successfully read the options file.
	// Our code can handle the default struct gracefully
	opts := &device.KuOptions{}
	configBytes, err := ioutil.ReadFile(filepath.Join(dbRootDir, ".adds/kobo-uncaged/config/ku.toml"))
	if err != nil {
		return opts, errors.Wrap(err, "error loading config file")
	}
	if err := toml.Unmarshal(configBytes, opts); err != nil {
		return opts, errors.Wrap(err, "error reading config file")
	}
	opts.Thumbnail.Validate()
	opts.Thumbnail.SetRezFilter()
	return opts, nil
}
func mainWithErrCode() returnCode {
	w, err := syslog.New(syslog.LOG_DEBUG, "KoboUNCaGED")
	if err == nil {
		log.SetOutput(w)
	}
	onboardMntPtr := flag.String("onboardmount", "/mnt/onboard", "If changed, specify the new new mountpoint of '/mnt/onboard'")
	sdMntPtr := flag.String("sdmount", "", "If changed, specify the new new mountpoint of '/mnt/sd'")
	mdPtr := flag.Bool("metadata", false, "Updates the Kobo DB with new metadata")

	flag.Parse()
	log.Println("Started Kobo-UNCaGED")
	log.Println("Reading options")
	opts, optErr := getUserOptions(*onboardMntPtr)
	log.Println("Creating KU object")
	k, err := device.New(*onboardMntPtr, *sdMntPtr, *mdPtr, opts)
	if err != nil {
		log.Print(err)
		return kuError
	}
	defer k.Close()
	if optErr != nil {
		k.Kup.Println(kuprint.Body, optErr.Error())
	}
	if *mdPtr {
		log.Println("Updating Metadata")
		k.Kup.Println(kuprint.Body, "Updating Metadata!")
		err = k.UpdateNickelDB()
		if err != nil {
			log.Print(err)
			return kuError
		}
		k.Kup.Println(kuprint.Body, "Metadata Updated!\n\nReturning to Home screen")
	} else {
		log.Println("Preparing Kobo UNCaGED!")
		ku := kunc.New(k)
		cc, err := uc.New(ku, true)
		if err != nil {
			log.Print(err)
			// TODO: Probably need to come up with a set of error codes for
			//       UNCaGED instead of this string comparison
			if err.Error() == "calibre server not found" {
				k.Kup.Println(kuprint.Body, "Calibre not found!\nHave you enabled the Calibre Wireless service?")
			}
			return kuError
		}
		log.Println("Starting Calibre Connection")
		err = cc.Start()
		if err != nil {
			if err.Error() == "no password entered" {
				k.Kup.Println(kuprint.Body, "No valid password found!")
				return kuPasswordError
			}
			log.Print(err)
			return kuError
		}

		if len(k.UpdatedMetadata) > 0 {
			k.Kup.Println(kuprint.Body, "Kobo-UNCaGED will restart automatically to update metadata")
			return kuSuccessRerun
		}
		k.Kup.Println(kuprint.Body, "Nothing more to do!\n\nReturning to Home screen")
	}

	return kuSuccessNoAction
}
func main() {
	os.Exit(int(mainWithErrCode()))
}
