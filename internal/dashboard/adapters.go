package dashboard

import (
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func mapConditions(in []gcprun.Condition) []views.ConditionVM {
	out := make([]views.ConditionVM, 0, len(in))
	for _, c := range in {
		out = append(out, views.ConditionVM{
			Type:  c.Type,
			State: c.State,
		})
	}
	return out
}