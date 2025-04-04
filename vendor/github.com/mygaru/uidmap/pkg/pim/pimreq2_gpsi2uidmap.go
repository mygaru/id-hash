package pim

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"time"
)

// GpsiRequest - requests from GPSI to Uidmap
type GpsiRequest []byte

func NewGpsiRequest(telcoID, partnerID, pimReqID uuid.UUID) GpsiRequest {
	return []byte(fmt.Sprintf(`{"telco_id": %q, "partner_id": %q, "pim_id": %q, "data": [`, telcoID, partnerID, pimReqID))
}

func (r *GpsiRequest) AddRow(telcoIdent, token []byte) {
	*r = append(*r, fmt.Sprintf(`[%q, %q],`, telcoIdent, token)...)
}

func (r *GpsiRequest) Form() {
	if (*r)[len(*r)-1] != '}' {
		(*r)[len(*r)-1] = ']'
		*r = append(*r, '}')
	}
}

func (r *GpsiRequest) Iterate(cb func(telcoIdent, token []byte) error) error {
	r.Form()
	val, err := fastjson.ParseBytes(*r)
	if err != nil {
		return err
	}

	dataArray := val.GetArray("data")

	for _, item := range dataArray {
		err := cb(item.GetStringBytes("0"), item.GetStringBytes("1"))
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *GpsiRequest) GetIDs() (telcoID, partnerID, pimReqID uuid.UUID, err error) {
	r.Form()
	telcoID, err = uuid.Parse(fastjson.GetString(*r, "telco_id"))
	if err != nil {
		return
	}

	partnerID, err = uuid.Parse(fastjson.GetString(*r, "partner_id"))
	if err != nil {
		return
	}

	pimReqID, err = uuid.Parse(fastjson.GetString(*r, "pim_id"))
	if err != nil {
		return
	}

	return
}

func SendGpsiRequest(r GpsiRequest, uidMapperURI string, timeout time.Duration) error {
	r.Form()
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	defer func() {
		fasthttp.ReleaseResponse(resp)
		fasthttp.ReleaseRequest(req)
	}()

	req.SetRequestURI(uidMapperURI + "/pim")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(r)

	err := fasthttp.DoTimeout(req, resp, timeout)
	if err != nil {
		return err
	}

	if resp.StatusCode() != fasthttp.StatusNoContent {
		return fmt.Errorf("error sending request to Uidmap (%s), got %d (wanted 204): %s", req.URI().String(), resp.StatusCode(), resp.Body())
	}

	return nil
}
