# Kobo-UNCaGED
Kobo-UNCaGED is a program for Kobo eink readers for connecting wirelessly to Calibre. It makes use of my [UNCaGED](https://github.com/shermp/UNCaGED) library. It is licenced under the AGPL-3.0 licence.

**Warning!!! This program is currently extremely unstable. Please only use it if you are helping to test and improve said stability**

## Build Steps

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
2. `cd ~/go/src/github.com/shermp/Kobo-UNCaGED` OR `cd ${GOPATH}/go/src/github.com/shermp/Kobo-UNCaGED` depending on how your Go environment was installed.
3. `sh build-ku.sh`. This will download and build all further requirements, and create the necessary directory structure.
4. Extract `./Build/KoboUncaged.zip` to the root of your Kobo user directory (`/mnt/onboard`).
5. Unplug, then reboot your Kobo.