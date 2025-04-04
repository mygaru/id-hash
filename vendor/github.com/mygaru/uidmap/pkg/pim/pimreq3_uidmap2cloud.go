package pim

import (
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"time"
)

var cloudURI = flag.String("cloudURI", "", "")

// UidmapRequest - requests from Uidmap to Cloud
type UidmapRequest []byte

func NewUidmapRequest(telcoID, partnerID, pimReqID uuid.UUID) UidmapRequest {
	return []byte(fmt.Sprintf(`{"telco_id": %q, "partner_id": %q, "pim_id": %q, "data": [`, telcoID, partnerID, pimReqID))
}

func (r *UidmapRequest) AddRow(ephID uuid.UUID, token []byte) {
	*r = append(*r, fmt.Sprintf(`[%q, %q],`, ephID, token)...)
}

func (r *UidmapRequest) Form() {
	if (*r)[len(*r)-1] != '}' {
		(*r)[len(*r)-1] = ']'
		*r = append(*r, '}')
	}
}

func (r *UidmapRequest) Iterate(cb func(ephID uuid.UUID, token []byte) error) error {
	r.Form()
	val, err := fastjson.ParseBytes(*r)
	if err != nil {
		return err
	}

	dataArray := val.GetArray("data")

	for _, item := range dataArray {
		eph, err := uuid.ParseBytes(item.GetStringBytes("0"))
		if err != nil {
			return err
		}

		err = cb(eph, item.GetStringBytes("1"))
		if err != nil {
			return err
		}
	}

	return nil
}

func SendUidampRequest(r UidmapRequest, cloudUri string, timeout time.Duration) error {
	r.Form()
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	defer func() {
		fasthttp.ReleaseResponse(resp)
		fasthttp.ReleaseRequest(req)
	}()

	req.SetRequestURI(cloudUri + "/pim")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(r)

	err := fasthttp.DoTimeout(req, resp, timeout)
	if err != nil {
		return err
	}

	if resp.StatusCode() != fasthttp.StatusNoContent {
		return fmt.Errorf("error sending request to Cloud (%s), got %d (wanted 204): %s", req.URI().String(), resp.StatusCode(), resp.Body())
	}

	return nil
}

func (r *UidmapRequest) GetIDs() (telcoID, partnerID, pimReqID uuid.UUID, err error) {
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
