package pim

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/valyala/fastjson"
	"gitlab.adtelligent.com/common/shared/util"
)

// MsisdnRequest - requests from DCR to GPSI
type MsisdnRequest struct {
	Data  []byte
	PimID uuid.UUID
}

func NewDcrMsisdnRequest(telcoID, partnerID, pimReqID uuid.UUID) *MsisdnRequest {
	return &MsisdnRequest{Data: []byte(fmt.Sprintf(`{"telco_id": %q, "partner_id": %q, "pim_id": %q, "data": [`, telcoID, partnerID, pimReqID)), PimID: pimReqID}
}

// Call Form() before sending requests!
// DO NOT USE IN CONCURRENT GOROUTINES!

func (r *MsisdnRequest) AddRow(msisdn string) {
	// add to data array [msisdn, uuid.New]
	r.Data = append(r.Data, fmt.Sprintf(`[%q, %q],`, msisdn, uuid.New())...)
}

func (r *MsisdnRequest) Form() {
	if (r.Data)[len(r.Data)-1] == '[' {
		r.Data = append(r.Data, "]}"...)
	} else if (r.Data)[len(r.Data)-1] != '}' {
		(r.Data)[len(r.Data)-1] = ']'
		r.Data = append(r.Data, '}')
	}
}

func (r *MsisdnRequest) Iterate(cb func(msisdn, token string) error) error {
	r.Form()
	val, err := fastjson.ParseBytes(r.Data)
	if err != nil {
		return err
	}

	dataArray := val.GetArray("data")

	for _, item := range dataArray {
		err := cb(
			util.UnsafeBytes2Str(item.GetStringBytes("0")),
			util.UnsafeBytes2Str(item.GetStringBytes("1")))
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *MsisdnRequest) GetIDs() (telcoID, partnerID, pimReqID uuid.UUID, err error) {
	r.Form()
	telcoID, err = uuid.Parse(fastjson.GetString(r.Data, "telco_id"))
	if err != nil {
		return
	}

	partnerID, err = uuid.Parse(fastjson.GetString(r.Data, "partner_id"))
	if err != nil {
		return
	}

	pimReqID, err = uuid.Parse(fastjson.GetString(r.Data, "pim_id"))
	if err != nil {
		return
	}

	return
}

/////////////////////////////////////////////

type PuidsResponse []byte

func NewPuidsResponse() PuidsResponse {
	return []byte(`{"data": [`)
}

func (r *PuidsResponse) Form() {
	if (*r)[len(*r)-1] == '[' {
		*r = append(*r, "]}"...)
	} else if (*r)[len(*r)-1] != '}' {
		(*r)[len(*r)-1] = ']'
		*r = append(*r, '}')
	}
}

func (r *PuidsResponse) AddRow(token, puid string) {
	*r = append(*r, fmt.Sprintf(`[%q, %q],`, puid, token)...)
}

func (r *PuidsResponse) Iterate(cb func(token, puid string) error) error {
	r.Form()
	val, err := fastjson.ParseBytes(*r)
	if err != nil {
		return err
	}

	dataArray := val.GetArray("data")

	for _, item := range dataArray {
		err := cb(
			util.UnsafeBytes2Str(item.GetStringBytes("1")),
			util.UnsafeBytes2Str(item.GetStringBytes("0")),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *PuidsResponse) GetAsMap() (map[string]string, error) {
	puidMap := make(map[string]string)
	err := r.Iterate(func(token, puid string) error {
		puidMap[token] = puid
		return nil
	})
	if err != nil {
		return nil, err
	}
	return puidMap, nil
}
