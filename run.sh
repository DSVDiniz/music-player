#!/usr/bin/env bash
CGO_CFLAGS="${CGO_CFLAGS} -DSUPPORT_FILEFORMAT_FLAC" go run *.go $@