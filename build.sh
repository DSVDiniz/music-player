#!/usr/bin/env bash
CGO_CFLAGS="${CGO_CFLAGS} -DSUPPORT_FILEFORMAT_FLAC" go build -o music-player *.go
sudo mv music-player /usr/bin/