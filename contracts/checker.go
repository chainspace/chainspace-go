package contracts // import "chainspace.io/prototype/contracts"

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/sbac"
)

type Checker struct {
	iD            string
	procedureName string
	addr          string
}

type trace struct {
	Dependencies    []trace       `json:"dependencies"`
	Inputs          []interface{} `json:"inputs"`
	Labels          [][]string    `json:"labels"`
	Outputs         []interface{} `json:"outputs"`
	Parameters      []interface{} `json:"parameters"`
	ReferenceInputs []interface{} `json:"referenceInputs"`
	Returns         []interface{} `json:"returns"`
}

func encodeToStrings(ls [][]byte) []string {
	out := make([]string, 0, len(ls))
	for _, v := range ls {
		out = append(out, base64.StdEncoding.EncodeToString(v))
	}

	return out
}

func makeTrace(inputs, refInputs, parameters, outputs, returns [][]byte, labels [][]string, traces []*sbac.Trace) trace {
	deps := []trace{}
	for _, t := range traces {
		t := t
		trace := makeTrace(
			t.InputObjects,
			t.InputReferences,
			t.Parameters,
			t.OutputObjects,
			t.Returns,
			sbac.StringsSlice(t.Labels).AsSlice(),
			t.Dependencies)
		deps = append(deps, trace)
	}

	return trace{
		Inputs:          unmarshalIfaceSlice(inputs),
		ReferenceInputs: unmarshalIfaceSlice(refInputs),
		Parameters:      unmarshalIfaceSlice(parameters),
		Outputs:         unmarshalIfaceSlice(outputs),
		Returns:         unmarshalIfaceSlice(returns),
		Labels:          labels,
		Dependencies:    deps,
	}
}

func unmarshalIfaceSlice(ls [][]byte) []interface{} {
	out := []interface{}{}
	for _, v := range ls {
		var val interface{}
		err := json.Unmarshal(v, &val)
		if err != nil {
			log.Fatal("unable to Unmarshal slice", fld.Err(err))
		}
		out = append(out, val)
	}
	return out
}

func (c Checker) Check(
	inputs, refInputs, parameters, outputs, returns [][]byte, labels [][]string, dependencies []*sbac.Trace) bool {
	body := makeTrace(inputs, refInputs, parameters, outputs, returns, labels, dependencies)
	bbody, _ := json.Marshal(body)
	payload := bytes.NewBuffer(bbody)

	req, err := http.NewRequest(http.MethodPost, c.addr, payload)
	if err != nil {
		log.Error("unable to crate http request", fld.Err(err))
		return false
	}
	ctx, cfunc := context.WithTimeout(context.Background(), time.Second)
	defer cfunc()
	req.WithContext(ctx)
	req.Header.Add("Content-type", "application/json")

	client := http.Client{
		Timeout: time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("unable to call checker", log.String("checker.id", c.iD), log.String("checker.procedure", c.procedureName), fld.Err(err))
		return false
	}

	bresp, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Error("unable to read checker response", log.String("checker.id", c.iD), log.String("checker.procedure", c.procedureName), fld.Err(err))
		return false
	}

	res := struct {
		Success bool `json:"success"`
	}{}
	err = json.Unmarshal(bresp, &res)
	if err != nil {
		log.Error("unable to unmarshal checker response", log.String("checker.id", c.iD), log.String("checker.procedure", c.procedureName), fld.Err(err))
		return false
	}
	return res.Success
}

func (c Checker) ContractID() string { return c.iD }

func (c Checker) Name() string {
	return c.procedureName
}

func NewCheckers(cfg *config.Contract) []Checker {
	u := path.Join(cfg.Addr, cfg.Name)
	checkers := []Checker{}

	for _, v := range cfg.Procedures {
		pth := path.Join(u, v)
		_u, _ := url.Parse(pth)
		checkers = append(checkers, Checker{cfg.Name, v, _u.String()})
	}

	return checkers
}

func NewDockerCheckers(cfg *config.DockerContract) []Checker {
	u, _ := url.Parse(fmt.Sprintf("http://0.0.0.0:%v", cfg.HostPort))
	u.Path = path.Join(u.Path, cfg.Name)
	checkers := []Checker{}

	for _, v := range cfg.Procedures {
		_u := *u
		_u.Path = path.Join(_u.Path, v)
		checkers = append(checkers, Checker{cfg.Name, v, _u.String()})
	}

	return checkers
}
