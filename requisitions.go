package nordigen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const requisiontsPath = "requisitions"
const linksPath = "links"

type RequisitionLinkRequest struct {
	AspspsId string `json:"aspsp_id"`
}

type RequisitionLinkResponse struct {
	Initiate string `json:"initiate"`
}

type Requisition struct {
	Redirect   string   `json:"redirect"`
	Reference  string   `json:"reference"`
	EnduserId  string   `json:"enduser_id"`
	Id         string   `json:"id"`
	Status     string   `json:"status"`
	Agreements []string `json:"agreements"`
	Accounts   []string `json:"accounts"`
}

func unexpectedResponse(expected int, resp *http.Response) error {
	return fmt.Errorf("expected %d status code: got %d %s", expected, resp.StatusCode, resp.Body)
}

func (c Client) CreateRequisition(r Requisition) (Requisition, error) {
	req := http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Path: strings.Join([]string{requisiontsPath, ""}, "/"),
		},
	}
	data, err := json.Marshal(r)

	if err != nil {
		return Requisition{}, nil
	}
	req.Body = io.NopCloser(bytes.NewBuffer(data))

	resp, err := c.c.Do(&req)

	if err != nil {
		return Requisition{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)


	if err != nil {
		return Requisition{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return Requisition{}, unexpectedResponse(http.StatusCreated, resp)
	}
	err = json.Unmarshal(body, &r)

	if err != nil {
		return Requisition{}, err
	}

	return r, nil
}

func (c Client) GetRequisition(id string) (r Requisition, err error) {
	req := http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Path: strings.Join([]string{requisiontsPath, id,""}, "/"),
		},
	}
	resp, err := c.c.Do(&req)

	if err != nil {
		return Requisition{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return Requisition{}, err
	}

	if resp.StatusCode != http.StatusOK{
		return Requisition{}, unexpectedResponse(http.StatusOK, resp)
	}
	err = json.Unmarshal(body, &r)

	if err != nil {
		return Requisition{}, err
	}

	return r, nil
}


func (c Client) CreateRequisitionLink(referenceId string, rl RequisitionLinkRequest) (RequisitionLinkResponse, error) {
	req := http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Path: strings.Join([]string{requisiontsPath, referenceId, linksPath, ""}, "/"),
		},
	}
	data, err := json.Marshal(rl)

	if err != nil {
		return RequisitionLinkResponse{}, nil
	}
	req.Body = io.NopCloser(bytes.NewBuffer(data))

	resp, err := c.c.Do(&req)

	if err != nil {
		return RequisitionLinkResponse{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return RequisitionLinkResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return RequisitionLinkResponse{}, unexpectedResponse(http.StatusOK, resp)
	}
	rr := RequisitionLinkResponse{}
	err = json.Unmarshal(body, &rr)

	if err != nil {
		return RequisitionLinkResponse{}, err
	}

	return rr, nil
}
