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

# We need those exported in the env for Go to pick them up
export CC
export CXX
export STRIP
export AR
export RANLIB
export GOOS
export GOARCH
export CGO_ENABLED

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

# List of pairs of files to appear in the final zip archive.
# The first element in each pair is the source file, the second
# is what the file should be renamed to
override ARCHIVE_FILES := \
	$(KU_BIN):$(ARC_KU_ROOT)/bin/ku \
	$(SQL_BIN):$(ARC_KU_ROOT)/bin/sqlite3 \
	$(NDB_ARCHIVE):$(ARC_KU_ROOT)/NickelDBus/ndb-kr.tgz \
	scripts/ku-lib.sh:$(ARC_KU_ROOT)/scripts/ku-lib.sh \
	scripts/ku-prereq-check.sh:$(ARC_KU_ROOT)/scripts/ku-prereq-check.sh \
	scripts/nm-start-ku.sh:$(ARC_KU_ROOT)/nm-start-ku.sh \
	config/nm-ku:$(ARC_ADDS_ROOT)/nm/kobo_uncaged

# Get a list of source files only from the above list
override ARCHIVE_SRCS := $(foreach pair,$(ARCHIVE_FILES),$(word 1,$(subst :, ,$(pair))))

override KU_SRC := $(wildcard kobo-uncaged/*.go kobo-uncaged/device/*.go kobo-uncaged/kunc/*.go kobo-uncaged/util/*.go)
# Gets the current version of the repository. This version gets embedded in the KU binary at compile time.
override KU_VERS := $(shell git describe --tags)

# This is the name of the sqlite archive and subdirectory.
override SQLITE_VER := sqlite-amalgamation-3380000
override SQLITE_SRC := $(DL_DIR)/$(SQLITE_VER)/shell.c $(DL_DIR)/$(SQLITE_VER)/sqlite3.c

# Rename multiple files in a zip file using zipnote. First arg is the zip file to update, the second arg
# is a list of filename pairs. Each pair is in the format <existing>:<new>
override zip_rename_files = printf "$(subst \n @,\n@,$(foreach pair,$(2),@ $(word 1,$(subst :, ,$(pair)))\n@=$(word 2,$(subst :, ,$(pair)))\n@ (comment above this line)\n))" | zipnote -w $(1)

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

$(KU_ARCHIVE): $(ARCHIVE_SRCS) | $(BUILD_DIR)
	rm -f $@ && \
	zip $@ $^ && \
	$(call zip_rename_files,$@,$(ARCHIVE_FILES))

$(NDB_ARCHIVE): | $(DL_DIR)
	wget -O $@ https://github.com/shermp/NickelDBus/releases/download/$(NDB_VER)/KoboRoot.tgz

$(KU_BIN): $(KU_SRC) | $(BUILD_DIR)
	go build -ldflags "-s -w -X main.kuVersion=$(KU_VERS)" -o $@ ./kobo-uncaged

$(SQL_BIN): $(SQLITE_SRC)
	$(CC) -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION -O2 $^ -o $@

$(SQLITE_SRC): $(DL_DIR)/$(SQLITE_VER).zip
	[ -f $@ ] || (cd $(dir $<) && unzip $(notdir $<))

$(DL_DIR)/$(SQLITE_VER).zip: | $(DL_DIR)
	wget -O $@ https://www.sqlite.org/2022/$(SQLITE_VER).zip

$(BUILD_DIR) $(DL_DIR):
	mkdir -p $@
