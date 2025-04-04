package main

import (
	"flag"
	"fmt"
	"github.com/mygaru/gpsi/cmd/gpsi/internal/pim"
	"github.com/valyala/fasthttp"
	"github.com/vharitonsky/iniflags"
	"gitlab.adtelligent.com/common/shared/httpauth"
	"gitlab.adtelligent.com/common/shared/httpserver"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/metric"
	"gitlab.adtelligent.com/common/shared/util"
)

func main() {
	iniflags.Parse()
	util.LogAllFlags()

	log.Infof("Initializing...")

	log.Infof("Initialized.")

	httpserver.Run(requestHandler, nil)
}

var (
	Requests            = metric.NewCounter("gpsiRequests")
	UnsupportedRequests = metric.NewCounter("gpsiUnsupportedRequests")
)

var writeLogUnsupportedError = flag.Bool("dmpCloudWriteLogUnsupportedError", true, "Unsupported http path Logger")

func requestHandler(ctx *fasthttp.RequestCtx) {
	path := ctx.Path()

	Requests.Inc()

	if !httpauth.IsAuthorized(ctx) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	if pim.Route(string(path), ctx) {
		return
	}

	switch string(path) {

	case "/egg", "/egg/":
		_, _ = fmt.Fprintf(ctx, `
      DMP GPSI
----------------------------------
	    /\_/\
	  =( °w° )=
	    ) - (  //
	   (__ __)//
----------------------------------
      All rights reserved

Rev: %s / %s
MyGaru Inc

`, log.GetBuildRevision(), log.GetBuildVersion())

	default:

		if *writeLogUnsupportedError {
			ctx.Logger().Printf("Unsupported http path requested: %q", path)
		}

		ctx.Error("Unsupported http path", fasthttp.StatusNotFound)
		UnsupportedRequests.Inc()
	}
}
