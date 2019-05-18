# Kobo-UNCaGED
Kobo-UNCaGED is a program for Kobo eink readers for connecting wirelessly to Calibre. It makes use of my [UNCaGED](https://github.com/shermp/UNCaGED) library. It is licenced under the AGPL-3.0 licence.

**Kobo-UNCaGED is currently ALPHA software. While I don't have any major issues with it, testing has been relatively limited, and I can't guarentee a problem free experience. If your ebook library is important to you, backup your Kobo user partition before use!**

## About
Kobo-UNCaGED runs on any Kobo with modern firmware (earliest version unknown). It is designed to be run from within the Kobo environment (Nickel), and allows you to connect your Kobo to Calibre using its wireless driver. This is the same connection/protocol that Calibre Companion (for Android) uses, and I decided why couldn't the rest of us join in the wireless fun?

KU works by using scripts to safely enter a fake USB mode. Once in this mode, the program runs, then safely exits back to your homescreen. It may additionally run a second time to update the Kobo database with additional book metadata.

### Supported Features
* Get list of current sideloaded books on device
* Update metadata on device
* Send new and replacement ebooks. Format support is the official list of supported formats such as epub, pdf, txt, rtf, html, mobi etc. kepub is also supported.
* Retrieve/read books from the device.
* Automatically set series metadata
* Remove books from device
* Generate library thumbnails for new books sent
* Connect to password protected calibre instances

Note, working with store-bought books are currently not supported. Also, KU will use and overwrite any existing metadata.calibre file. This could cause some data "loss" in that the metadata cache will lose any info on non-sideloaded books.

## Installing/running
Kobo-UNCaGED is designed to be launched from within the Kobo software (nickel) using launchers such as fmon or kfmon. Launching using other systems such as KSM is not supported, as they have not been tested, or likely to work with the current launch scripts.

### Installation
1. Install an application launcher. I recommend [kfmon](https://github.com/NiLuJe/kfmon), as that is what I have tested with, and have provided a configuration file for. Follow the installation instructions for the launcher you will use.
2. Download the latest release zip archive from https://github.com/shermp/Kobo-UNCaGED/releases
3. Unzip the contents of the archive directly onto the root directory of your kobo partion (when connected with USB). All files should be copied to their appropriate location.
4. (optional) navigate to `.adds/kobo-uncaged/config`, and copy `ku.toml.default` to `ku.toml`, and make desired configuration changes. If you don't perform this step, the startup script will do so automatically on first run.
5. Disconnect/eject your Kobo, and restart it.

### Upgrading
Download the new release zip archive, and extract. There should be no need to restart your Kobo.

### Firmware upgrade notes
If your Kobo firmware is updated, you will most likely need to reinstall your launcher of choice. Kobo-UNCaGED should not need to be reinstalled however.

### Running (with kfmon)
1. Make sure you have enabled the Wireless connection in Calibre (`Connect/Share`>`Start wireless device connection`). Also ensure that your Kobo can connect to a wireless network.
2. Search for, or browse to the `Kobo-UNCaGED` image on your Kobo and open it. You should see a brief message at the bottom of the screen that confirms it has started `start-ku.sh`
3. KU will fake a USB connection, and **automatically** press the `connect` button for you. It will then try to connect to a Wifi network, then connect to Calibre.
4. At this point, you can use Calibre to send/receive/update/remove books.
5. When you are finished, **eject** the wireless device from calibre, as you would a USB device.
6. You will be automatically returned to the home screen. Depending on what actions you took while using Calibre, it may restart automatically to update metadata.

Have Fun!

## Build Steps

If you want to build Kobo-UNCaGED for yourself, this is how you do it.

It is highly recommended to build Kobo-UNCaGED on a Linux based system. It will save you a lot of grief, and all instructions below will be for Linux.

### Prerequisites

Kobo-UNCaGED requires the following prerequisites to build correctly:

* [Go](https://golang.org/doc/install) (Minimum version unknown, but 1.10+ should be safe to use)
* [ARM Cross Compiler](https://github.com/koreader/koxtoolchain) is required, as some of the libraries required use CGO. We will also be compiling FBink. The linked toolchain by the KOReader developers is recommended. Note that this toolchain takes a LONG time to setup (40-50 minutes on my VM)
* Standard tools such as git, tar, zip, wget, make
* The shell/environment variable `CROSS_TC` is set to the name of your cross compiler eg: `arm-kobo-linux-gnueabihf`, and that `${CROSS_TC}-gcc` etc are in your PATH.
* [kfmon](https://github.com/NiLuJe/kfmon) installed on your Kobo.

### Building

Once the prerequisites are met, follow the next steps:

1. `go get github.com/shermp/Kobo-UNCaGED/kobo-uncaged/`
2. `cd ~/go/src/github.com/shermp/Kobo-UNCaGED` OR `cd ${GOPATH}/src/github.com/shermp/Kobo-UNCaGED` depending on how your Go environment was installed.
3. `sh build-ku.sh`. This will download and build all further requirements, and create the necessary directory structure.
4. Extract `./Build/KoboUncaged.zip` to the root of your Kobo user directory (`/mnt/onboard`).
5. Unplug, then reboot your Kobo.

### Developing

To help with development, it's recommended that you try the following: 
1. Download the repository using `go get` as above
2. Change to the source directory (as in step 2 above)
3. Fork the repository on Github
4. In the local repository, add your fork as a remote, eg: `git remote add remote_name fork_url`
5. Push changes to your fork, eg: `git push remote_name`
6. Open a PR if you want to submit your changes back