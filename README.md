# Kobo-UNCaGED
**WARNING: This might not work on the new Kobo Clara Colour/BW or Libra Colour!**

Kobo-UNCaGED is a program for Kobo eink readers for connecting wirelessly to Calibre. It makes use of my [UNCaGED](https://github.com/shermp/UNCaGED) library. It is licenced under the AGPL-3.0 licence.

**While I don't have any major issues with Kobo-UNCaGED, testing has been relatively limited, and I can't guarantee a problem-free experience. If your ebook library is important to you, backup your Kobo user partition before use!**

**Very large ebook libraries on your Kobo will likely cause issues. If you have thousands of books on your device, proceed with caution!**

## About
Kobo-UNCaGED runs on any Kobo with firmware 4.13.12638 or newer. It is designed to be run from within the Kobo environment (Nickel), and allows you to connect your Kobo to Calibre using its wireless driver. This is the same connection/protocol that Calibre Companion (for Android) uses, and I decided why couldn't the rest of us join in the wireless fun?

### Supported Features
* Get list of current sideloaded books on device
* Update metadata on device
* Send new and replacement ebooks. Format support is the official list of supported formats such as epub, pdf, txt, rtf, html, mobi etc. kepub is also supported.
* Retrieve/read books from the device
* Automatically set series metadata
* Remove books from device
* Generate library thumbnails for new books sent
* Connect to password protected calibre instances
* Choose which Calibre instance to connect to if multiple are found on the network
* Set Kobo subtitle entry from a standard or custom column (with formatting)
* Directly connect to a host/port, to bypass autodiscovery

Note: Working with store-bought books is currently not supported. Also, KU will use and overwrite any existing metadata.calibre file. This could cause some data "loss" in that the metadata cache will lose any info on non-sideloaded books.

## Installing/running
Kobo-UNCaGED is designed to be launched from within the Kobo software (nickel) using NickelMenu. The current version does not support launching KU from any other launcher such as kfmon, fmon, or Kobo Start Manager (KSM).

KU also requires [NickelDBus](https://github.com/shermp/NickelDBus). For your convenience, KU installs it for you on first start, if you do not already have it installed.

### Upgrading from < 0.5 to 0.5.0+
0.5.0 introduces a large number of changes compared to earlier versions of Kobo UNCaGED. Please take note of the following when upgrading:

* NickelMenu is now the preferred (and only supported) method of launching KU. Launching KU with kfmon/fmon will cease to work after upgrading. It is advised that you delete `Kobo-UNCaGED.png` and `.adds/kfmon/config/kobo-uncaged.ini`. Feel free to uninstall kfmon if it is no longer required.
* Because NickelMenu is used, you will need to install it before you can start KU.
* `.adds/kobo-uncaged/config/ku.toml` is no longer used. All configuration is now handled (and saved) via the Web UI.

### Installation
0. Ensure you are running firmware 4.13.12638 or newer. Kobo UNCaGED will refuse to launch if it detects an earlier firmware version. 
1. Install [NickelMenu](https://github.com/pgaskin/NickelMenu/releases) if you don't already have it. Version 0.3.x or newer is highly recommended, although version 0.2.x should work as well.
2. Download the latest release zip archive from https://github.com/shermp/Kobo-UNCaGED/releases
3. Unzip the contents of the archive directly onto the root directory of your kobo partion (when connected with USB). All files should be copied to their appropriate location.
5. Disconnect/eject your Kobo, and launch the `Kobo UNCaGED` entry from NickelMenu.
6. If it's not already installed, NickelDBus will be installed on first start. If this is required, your Kobo will reboot again.

### Upgrading
Download the new zip archive, and extract. There should be no need to restart your Kobo.

### Firmware upgrade notes
Kobo UNCaGED should survive firmware updates, as does NickelMenu.

### Uninstalling
Connect your Kobo to the computer over USB. Delete the `.adds/kobo-uncaged` directory and the `.adds/nm/kobo_uncaged` NickelMenu config file. 

If you want to uninstall NickelDBus as well, delete the `.adds/nickeldbus` file. Note, you will need to reboot your Kobo to complete the NickelDBus removal.

### Running
1. Make sure you have enabled the Wireless connection in Calibre (`Connect/Share`>`Start wireless device connection`). Also ensure that your Kobo can connect to a wireless network.
2. Launch `Kobo UNCaGED` from the main menu (using NickelMenu).
3. KU will open the web browser. If required, you will be prompted to enable/connect to WiFi. You have a minute to connect to Wifi and let the browser open before KU times out and exits.
4. The browser opens a configuration screen to set options. Options are saved if you make any changes. Press the `Start` button to connect to Calibre.
    * The config page allows you to set a host to directly connect to as an alternative of autodiscovery. Press the **+** button to add a host, and the **-** button to remove the currently selected host.
5. If there are multiple Calibre instances on the network, KU will provide a list for you to select one. If the Calibre instance is password protected, you will be prompted to enter the password. The password will be saved for future connections.
6. At this point, you can use Calibre to send/receive/update/remove books. 
    * When connected, you can also set what Calibre column (if any) to use to populate the 'subtitle' field.
    * Kobo UNCaGED can (mostly) parse the display format for a column if it is set in Calibre
7. When you are finished, **eject** the wireless device from calibre, as you would a USB device. Alternatively, you can press the `disconnect` button in KU
8. KU will trigger the content import process, and update metadata if required.
9. A **Finished** dialog box will show when all content has been imported and metadata updated. Press **Continue** to start reading. Please don't attempt to interact with your Kobo untill this dialog shows.

Have Fun!

## Build Steps

If you want to build Kobo-UNCaGED for yourself, this is how you do it.

It is highly recommended to build Kobo-UNCaGED on a Linux based system. It will save you a lot of grief, and all instructions below will be for Linux.

### Prerequisites

Kobo-UNCaGED requires the following prerequisites to build correctly:

* [Go](https://golang.org/doc/install) Go 1.14+ is required
* [ARM Cross Compiler](https://github.com/koreader/koxtoolchain) is required, as some of the libraries required use CGO. We will also be compiling SQLite. The linked toolchain by the KOReader developers is recommended. Note that this toolchain takes a LONG time to setup (40-50 minutes on my VM)
* Standard tools such as git, tar, zip, unzip, wget, make
* The shell/environment variable `CROSS_TC` or `CROSS_COMPILE` is set to the name of your cross compiler eg: `arm-kobo-linux-gnueabihf`, and that `${CROSS_TC}-gcc`/`${CROSS_COMPILE}gcc` etc are in your PATH. Alternatively, the variable `CC` can be set to `arm-kobo-linux-gnueabihf`.

### Building

Once the prerequisites are met, follow the next steps:

1. `git clone github.com/shermp/Kobo-UNCaGED`
2. `cd Kobo-UNCaGED`
3. Run `make`. This will download and build all further requirements, and create the necessary directory structure.
4. Extract `./build/Kobo-UNCaGED.zip` to the root of your Kobo user directory (`/mnt/onboard`).
5. Unplug your Kobo.

You can run `make clean` to remove all build artifacts except downloaded files. `make cleanall` will also remove downloads.

### Developing

To help with development, it's recommended that you try the following: 
1. Download the repository as above
2. Change to the source directory (as in step 2 above)
3. Fork the repository on Github
4. In the local repository, add your fork as a remote, eg: `git remote add remote_name fork_url`
5. Push changes to your fork, eg: `git push remote_name`
6. Open a PR if you want to submit your changes back
