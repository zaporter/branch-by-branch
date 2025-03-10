package experiment

import (
	"context"
	"regexp"

	"github.com/zaporter/branch-by-branch/orchestrator/lambda"
)

type instanceReservationReq struct {
	Type        string `json:"type"`
	Count       int    `json:"count"`
	RegionMatch string `json:"region_match"`
	SetupCmd    string `json:"setup_cmd"`
}

func (r *instanceReservationReq) toLambdaInstanceRequest() *lambda.InstanceRequest {
	return &lambda.InstanceRequest{
		Type:        regexp.MustCompile(r.Type),
		Count:       r.Count,
		RegionMatch: regexp.MustCompile(r.RegionMatch),
		SetupCmd:    r.SetupCmd,
	}
}

func reserveInstances(ctx context.Context, reqs map[string]instanceReservationReq) error {
	lambdaReqs := make(map[string]*lambda.InstanceRequest)
	for key, req := range reqs {
		lambdaReqs[key] = req.toLambdaInstanceRequest()
	}
	return lambda.ReserveInstances(ctx, lambdaReqs)
}
