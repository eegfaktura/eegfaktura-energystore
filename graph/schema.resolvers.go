package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"github.com/99designs/gqlgen/graphql"
	"github.com/golang/glog"
)

// SingleUpload is the resolver for the singleUpload field.
func (r *mutationResolver) SingleUpload(ctx context.Context, tenant string, sheet string, file graphql.Upload) (bool, error) {
	glog.Infof("START UPLOAD: %+v %+v", tenant, sheet)
	err := excel.ImportFile(tenant, file.Filename, sheet, file.File)
	return err == nil, err
}

// LastEnergyDate is the resolver for the lastEnergyDate field.
func (r *queryResolver) LastEnergyDate(ctx context.Context, tenant string) (string, error) {
	return services.GetLastEnergyEntry(tenant)
}

// Report is the resolver for the report field.
func (r *queryResolver) Report(ctx context.Context, tenant string, year int, segment int, period string) (*model.EegEnergy, error) {
	var err error
	energy := &model.EegEnergy{}

	if energy, err = calculation.EnergyReport(tenant, year, segment, period); err != nil {
		return energy, err
	}

	return energy, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
