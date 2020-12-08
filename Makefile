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
	$(error Cross compiler not detected.)
endif

override GOOS := linux
override GOARCH := arm
override CGO_ENABLED := 1

override KU_ARCHIVE := build/Kobo-UNCaGED.zip
override ADDS_ROOT := build/onboardroot/.adds
override KU_ROOT := $(ADDS_ROOT)/kobo-uncaged

override KU_BIN := $(KU_ROOT)/bin/kobo-uncaged
override SQL_BIN := $(KU_ROOT)/bin/sqlite3
override NDB_VER := 0.1.0
override NDB_ARCHIVE := $(KU_ROOT)/NickelDBus/ndb-kr.tgz
override KU_SCRIPTS := $(KU_ROOT)/scripts/ku-lib.sh $(KU_ROOT)/scripts/ku-prereq-check.sh
override KU_STATIC := $(KU_ROOT)/static/html_input.css $(KU_ROOT)/static/ku.css $(KU_ROOT)/static/ku.js
override KU_TMPL := $(KU_ROOT)/templates/kuPage.tmpl
override KU_START := $(KU_ROOT)/nm-start-ku.sh
override NM_CFG := $(ADDS_ROOT)/nm/kobo_uncaged

override BULD_FILES := $(KU_BIN) $(SQL_BIN) $(NDB_ARCHIVE) $(KU_SCRIPTS) $(KU_STATIC) $(KU_TMPL) $(KU_START) $(NM_CFG)

override KU_SRC := $(shell find kobo-uncaged -name '*.go')
override KU_VERS := $(shell git describe --tags)

override DL_DIR := build/downloads

override SQLITE_VER := sqlite-amalgamation-3340000
override SQLITE_SRC := $(DL_DIR)/$(SQLITE_VER)/shell.c $(DL_DIR)/$(SQLITE_VER)/sqlite3.c

override BUILD_DIRS := $(sort $(dir $(BULD_FILES))) $(DL_DIR) $(KU_ROOT)/config

$(info $(shell for d in $(BUILD_DIRS); do mkdir -p "$$d"; done))

.PHONY: all clean cleanall

all: $(KU_ARCHIVE)

clean:
	rm $(KU_ARCHIVE)
	rm -rf build/onboardroot/

cleanall:
	rm -rf build/

$(KU_ARCHIVE): $(BULD_FILES)
	cd build/onboardroot && zip -r ../$(notdir $@) .

$(NDB_ARCHIVE): $(DL_DIR)/ndb-$(NDB_VER).tgz
	cp $< $@

$(DL_DIR)/ndb-$(NDB_VER).tgz:
	wget -O $@ https://github.com/shermp/NickelDBus/releases/download/$(NDB_VER)/KoboRoot.tgz

$(KU_SCRIPTS):
	cp scripts/$(notdir $@) $@

$(KU_STATIC):
	cp kobo-uncaged/static/$(notdir $@) $@

$(KU_TMPL):
	cp kobo-uncaged/templates/$(notdir $@) $@

$(KU_START):
	cp scripts/$(notdir $@) $@

$(NM_CFG):
	cp config/nm-ku $@

$(KU_BIN): $(KU_SRC)
	go build -ldflags "-s -w -X main.kuVersion=$(KU_VERS)" -o $@ ./kobo-uncaged

$(SQL_BIN): $(SQLITE_SRC)
	$(CC) -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION -O2 $^ -o $@

$(SQLITE_SRC): $(DL_DIR)/$(SQLITE_VER).zip
	[ -f $@ ] || (cd $(dir $<) && unzip $(notdir $<))

$(DL_DIR)/$(SQLITE_VER).zip:
	wget -O $@ https://www.sqlite.org/2020/$(SQLITE_VER).zip

$(BUILD_DIRS):
	mkdir -p $@
