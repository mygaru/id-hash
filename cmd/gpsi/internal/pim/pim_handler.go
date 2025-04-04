package pim

import (
	"flag"
	"fmt"
	"github.com/mygaru/gpsi/cmd/gpsi/internal/core"
	"github.com/mygaru/uidmap/pkg/pim"
	"github.com/valyala/fasthttp"
	"gitlab.adtelligent.com/common/shared/log"
	"time"
)

var (
	uidMapUri  = flag.String("uidMapUri", "http://localhost:8022", "Uidmap URI")
	pimTimeout = flag.Duration("pimTimeout", 5*time.Minute, "Timeout for sending PIM batch to uidmap")
)

func Route(path string, ctx *fasthttp.RequestCtx) bool {

	switch path {
	case "/pim", "/pim/":
		HandlerProcessMsisdnRequest(ctx)

	default:
		return false
	}

	return true
}

func HandlerProcessMsisdnRequest(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		return
	}

	// todo check if ctx.user value is parter id

	dcrBatch := pim.MsisdnRequest{Data: ctx.PostBody()}

	telcoID, partnerID, pimReqID, err := dcrBatch.GetIDs()
	if err != nil {
		ctx.Error("failed to get parse body", fasthttp.StatusBadRequest)
		return
	}

	log.Infof("Got PIM batch, id=%s, telco=%s, partner=%s", pimReqID, telcoID, partnerID)

	log.Debugf("%s", dcrBatch.Data)

	gpsi2Uidmap := pim.NewGpsiRequest(telcoID, partnerID, pimReqID)
	gpsi2Uidmap, err = core.ProcessPimBatch(dcrBatch, gpsi2Uidmap)
	if err != nil {
		ctx.Error(fmt.Sprintf("error while processing request, err = %v", err), fasthttp.StatusInternalServerError)
		return
	}

	log.Debugf("%s", gpsi2Uidmap)
	err = pim.SendGpsiRequest(gpsi2Uidmap, *uidMapUri, *pimTimeout)
	if err != nil {
		ctx.Error(fmt.Sprintf("failed to send PIM request: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
