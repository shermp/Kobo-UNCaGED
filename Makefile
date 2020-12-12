# 'Borrowed' from FBInk Makefile :) 
ifdef CROSS_TC
	# NOTE: If we have a CROSS_TC toolchain w/ CC set to Clang,
	#       assume we know what we're doing, and that everything is setup the right way already (i.e., via x-compile.sh tc env clang)...
	ifneq "$(CC)" "clang"
		CC:=$(CROSS_TC)-gcc
		CXX:=$(CROSS_TC)-g++
		STRIP:=$(CROSS_TC)-strip
		# NOTE: This relies on GCC plugins!
		#       Enforce AR & RANLIB to point to their real binary and not the GCC wrappers if your TC doesn't support that!
		AR:=$(CROSS_TC)-gcc-ar
		RANLIB:=$(CROSS_TC)-gcc-ranlib
	endif
else ifdef CROSS_COMPILE
	CC:=$(CROSS_COMPILE)cc
	CXX:=$(CROSS_COMPILE)cxx
	STRIP:=$(CROSS_COMPILE)strip
	AR:=$(CROSS_COMPILE)gcc-ar
	RANLIB:=$(CROSS_COMPILE)gcc-ranlib
else ifneq (arm-kobo-linux-gnueabihf-gcc, $(CC))
    $(error Cross compiler not detected)
endif

# Set required Go environment variables for building on Kobo devices
override GOOS := linux
override GOARCH := arm
override CGO_ENABLED := 1

# Set required target files and directories
override BUILD_DIR := build
override DL_DIR := $(BUILD_DIR)/downloads
override KU_ARCHIVE := $(BUILD_DIR)/Kobo-UNCaGED.zip
override ARC_ADDS_ROOT := .adds
override ARC_KU_ROOT := $(ARC_ADDS_ROOT)/kobo-uncaged

override KU_BIN := $(BUILD_DIR)/ku
override SQL_BIN := $(BUILD_DIR)/sqlite3
override NDB_VER := 0.1.0
override NDB_ARCHIVE := $(DL_DIR)/ndb-$(NDB_VER).tgz
override KU_SCRIPTS := scripts/ku-lib.sh scripts/ku-prereq-check.sh
override KU_STATIC := kobo-uncaged/static/html_input.css kobo-uncaged/static/ku.css kobo-uncaged/static/ku.js
override KU_TMPL := kobo-uncaged/templates/kuPage.tmpl
override KU_START := scripts/nm-start-ku.sh
override NM_CFG := config/nm-ku

override KU_SRC := $(wildcard kobo-uncaged/*.go kobo-uncaged/device/*.go kobo-uncaged/kunc/*.go kobo-uncaged/util/*.go)
# Gets the current version of the repository. This version gets embedded in the KU binary at compile time.
override KU_VERS := $(shell git describe --tags)

# This is the name of the sqlite archive and subdirectory.
override SQLITE_VER := sqlite-amalgamation-3340000
override SQLITE_SRC := $(DL_DIR)/$(SQLITE_VER)/shell.c $(DL_DIR)/$(SQLITE_VER)/sqlite3.c

# Rename a single file in a zip file using zipnote. First arg is the zip file to update, the second arg
# is the file to rename, the third arg is the renamed filename/path
override zip_rename_single = printf "@ $(2)\n@=$(3)\n" | zipnote -w $(1)
# Rename multiple files in a zip file using zipnote. First arg is the zip file to update, the second arg
# is the list of files to rename, the third arg is an optional directory prefix for the renamed files
override zip_rename_multiple = printf "$(subst \n @,\n@,$(foreach file,$(2),@ $(file)\n@=$(dir $(3)/)$(notdir $(file))\n@ (comment above this line)\n))" | zipnote -w $(1)

.PHONY: all clean cleanall

all: $(KU_ARCHIVE)

clean:
	rm -f $(KU_ARCHIVE) $(KU_BIN) $(SQL_BIN)

cleanall: clean
	rm -f $(DL_DIR)/$(SQLITE_VER)/*
	rm -df $(DL_DIR)/$(SQLITE_VER)
	rm -f $(DL_DIR)/*
	rm -df $(DL_DIR)
	rm -df build

$(KU_ARCHIVE): $(KU_BIN) $(SQL_BIN) $(NDB_ARCHIVE) $(KU_SCRIPTS) $(KU_STATIC) $(KU_TMPL) $(KU_START) $(NM_CFG) | $(BUILD_DIR)
	zip $@ $^ && \
	$(call zip_rename_single,$@,$(KU_BIN),$(ARC_KU_ROOT)/bin/ku) && \
	$(call zip_rename_single,$@,$(SQL_BIN),$(ARC_KU_ROOT)/bin/sqlite3) && \
	$(call zip_rename_single,$@,$(NDB_ARCHIVE),$(ARC_KU_ROOT)/NickelDBus/ndb-kr.tgz) && \
	$(call zip_rename_multiple,$@,$(KU_SCRIPTS),$(ARC_KU_ROOT)/scripts) && \
	$(call zip_rename_multiple,$@,$(KU_STATIC),$(ARC_KU_ROOT)/static) && \
	$(call zip_rename_multiple,$@,$(KU_TMPL),$(ARC_KU_ROOT)/templates) && \
	$(call zip_rename_single,$@,$(KU_START),$(ARC_KU_ROOT)/$(notdir $(KU_START))) && \
	$(call zip_rename_single,$@,$(NM_CFG),$(ADDS_ROOT)/nm/kobo_uncaged)

$(NDB_ARCHIVE): | $(DL_DIR)
	wget -O $@ https://github.com/shermp/NickelDBus/releases/download/$(NDB_VER)/KoboRoot.tgz

$(KU_BIN): $(KU_SRC) | $(BUILD_DIR)
	go build -ldflags "-s -w -X main.kuVersion=$(KU_VERS)" -o $@ ./kobo-uncaged

$(SQL_BIN): $(SQLITE_SRC)
	$(CC) -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION -O2 $^ -o $@

$(SQLITE_SRC): $(DL_DIR)/$(SQLITE_VER).zip
	[ -f $@ ] || (cd $(dir $<) && unzip $(notdir $<))

$(DL_DIR)/$(SQLITE_VER).zip: | $(DL_DIR)
	wget -O $@ https://www.sqlite.org/2020/$(SQLITE_VER).zip

$(BUILD_DIR) $(DL_DIR):
	mkdir -p $@
