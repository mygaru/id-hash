package pim

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mygaru/id-hash/cmd/id-hash/internal/core"
	"github.com/valyala/fasthttp"
	"log"
	"time"
)

// EXAMPLE IMPLEMENTATION

var (
	uidMapAddr = flag.String("uidMapAddr", "http://localhost:8022", "Uidmap Address")
	pimTimeout = flag.Duration("pimTimeout", 5*time.Minute, "Timeout for sending PIM batch to uidmap")
)

func HandlerProcessMsisdnRequest(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		return
	}

	// 2. Parse request body data into expected batch. If this fails, return 400.
	var batch struct {
		TelcoID   string      `json:"telco_id"`
		PartnerID string      `json:"partner_id"`
		PimID     string      `json:"pim_id"`
		Data      [][2]string `json:"data"` // [phoneNumber, token]
	}

	if err := json.Unmarshal(ctx.PostBody(), &batch); err != nil {
		ctx.Error("failed to parse JSON body", fasthttp.StatusBadRequest)
		return
	}

	log.Printf("Got PIM batch telco=%q pim=%q pid=%q", batch.TelcoID, batch.PimID, batch.PartnerID)

	// 3. Validate PartnerID matches X-ClientID
	clientID := string(ctx.Request.Header.Peek("X-ClientID"))
	if clientID != batch.PartnerID {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	// 4. Process the mapping from phone number -> telco ident value
	mappedData, err := core.ProcessPimBatch(batch.Data)
	if err != nil {
		ctx.Error(fmt.Sprintf("failed to process batch: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	// 5. Build request body to send to UID Mapper
	uidMapperPayload := map[string]interface{}{
		"telco_id":   batch.TelcoID,
		"partner_id": batch.PartnerID,
		"pim_id":     batch.PimID,
		"data":       mappedData,
	}

	payloadBytes, err := json.Marshal(uidMapperPayload)
	if err != nil {
		ctx.Error(fmt.Sprintf("failed to marshal payload: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	// 6. Send request to UID Mapper
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(fmt.Sprintf("%s/pim", *uidMapAddr))
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ClientID", clientID)
	req.SetBody(payloadBytes)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	// 7. Process UID Mapper response
	if err := fasthttp.DoTimeout(req, resp, *pimTimeout); err != nil {
		ctx.Error(fmt.Sprintf("failed to send request to UID Mapper: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	if resp.StatusCode() != fasthttp.StatusNoContent {
		ctx.Error(fmt.Sprintf("UID Mapper returned unexpected status: %d", resp.StatusCode()), fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
