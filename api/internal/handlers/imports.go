package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/csvimport"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func openapiEmail(s string) openapi_types.Email { return openapi_types.Email(s) }

// ImportSplitwise parses a Splitwise CSV export and either previews the
// result (dry_run=true) or commits it (dry_run=false). See the OpenAPI spec
// for the full contract; the security-sensitive bits live in the service.
func (s *Server) ImportSplitwise(c *gin.Context) {
	s.importCSV(c, s.Imports.Run)
}

// ImportDoTheSplit is the dothesplit-flavored counterpart to
// ImportSplitwise. The request shape is identical; the only change is
// the parser the service uses (which understands the richer header
// produced by the export endpoint).
func (s *Server) ImportDoTheSplit(c *gin.Context) {
	s.importCSV(c, s.Imports.RunDoTheSplit)
}

// importCSV is the shared body of both import handlers; it differs only
// in which Importer.Run* method is called via `run`.
func (s *Server) importCSV(c *gin.Context, run func(context.Context, uuid.UUID, service.ImportSplitwiseInput) (service.ImportSplitwiseResult, error)) {
	u := middleware.User(c)
	var req apigen.ImportSplitwiseRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	if len(req.Members) < csvimport.MinUsers || len(req.Members) > csvimport.MaxUsers {
		writeErr(c, http.StatusBadRequest, "bad_request", "members count out of range")
		return
	}
	members := make([]service.ImportSplitwiseMember, len(req.Members))
	for i, m := range req.Members {
		members[i] = service.ImportSplitwiseMember{CSVName: m.CsvName, Email: string(m.Email)}
	}
	in := service.ImportSplitwiseInput{
		CSV:             req.Csv,
		GroupName:       req.GroupName,
		DefaultCurrency: req.DefaultCurrency,
		Members:         members,
	}
	if req.DryRun != nil {
		in.DryRun = *req.DryRun
	}

	res, err := run(c.Request.Context(), u.ID, in)
	switch {
	case errors.Is(err, csvimport.ErrCSVTooLarge),
		errors.Is(err, csvimport.ErrCSVBadHeader),
		errors.Is(err, csvimport.ErrCSVNoRows),
		errors.Is(err, csvimport.ErrCSVTooMany),
		errors.Is(err, csvimport.ErrCSVFieldLen):
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	case err != nil:
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	skipped := res.Skipped
	if skipped == nil {
		skipped = []string{}
	}
	balances := make([]apigen.ImportSplitwiseBalance, len(res.Balances))
	for i, b := range res.Balances {
		balances[i] = apigen.ImportSplitwiseBalance{CsvName: b.CSVName, NetCents: b.NetCents}
	}
	csvCurrencies := res.CSVCurrencies
	if csvCurrencies == nil {
		csvCurrencies = []string{}
	}
	out := apigen.ImportSplitwiseResponse{
		GroupName:       res.GroupName,
		DefaultCurrency: res.DefaultCurrency,
		ExpenseCount:    res.ExpenseCount,
		SettlementCount: res.SettlementCount,
		SkippedCount:    res.SkippedCount,
		Skipped:         skipped,
		Balances:        balances,
		CsvCurrencies:   csvCurrencies,
	}
	out.Members = make([]apigen.ImportSplitwiseMember, len(res.Members))
	for i, m := range res.Members {
		out.Members[i] = apigen.ImportSplitwiseMember{CsvName: m.CSVName, Email: openapiEmail(m.Email)}
	}
	out.Preview = make([]apigen.ImportSplitwisePreviewRow, 0, len(res.Preview))
	for _, p := range res.Preview {
		t, _ := p.IncurredAt.(time.Time)
		out.Preview = append(out.Preview, apigen.ImportSplitwisePreviewRow{
			Description:  p.Description,
			IncurredAt:   t,
			AmountCents:  p.AmountCents,
			Currency:     p.Currency,
			CategorySlug: p.CategorySlug,
			PayerCsvName: p.PayerCSVName,
		})
	}
	out.SettlementPreview = make([]apigen.ImportSplitwiseSettlementPreview, 0, len(res.SettlementPreview))
	for _, p := range res.SettlementPreview {
		t, _ := p.SettledAt.(time.Time)
		out.SettlementPreview = append(out.SettlementPreview, apigen.ImportSplitwiseSettlementPreview{
			Note:        p.Note,
			SettledAt:   t,
			AmountCents: p.AmountCents,
			Currency:    p.Currency,
			FromCsvName: p.FromCSVName,
			ToCsvName:   p.ToCSVName,
		})
	}
	if res.GroupID != nil {
		out.GroupId = res.GroupID
		c.JSON(http.StatusCreated, out)
		return
	}
	c.JSON(http.StatusOK, out)
}
