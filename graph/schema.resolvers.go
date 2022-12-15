package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"at.ourproject/energystore/excel"
	"context"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/model"
	"github.com/99designs/gqlgen/graphql"
)

// SingleUpload is the resolver for the singleUpload field.
func (r *mutationResolver) SingleUpload(ctx context.Context, tenant string, sheet string, file graphql.Upload) (bool, error) {
	err := excel.ImportFile(tenant, file.Filename, sheet, file.File)
	return err == nil, err
}

// Eeg is the resolver for the eeg field.
func (r *queryResolver) Eeg(ctx context.Context, name string, year int, month *int, function *string) (*model.EegEnergy, error) {
	var err error
	energy := &model.EegEnergy{}

	var fc string
	if function == nil {
		fc = ""
	} else {
		fc = *function
	}

	var m int
	if month == nil {
		m = 0
	} else {
		m = *month
	}

	if energy, err = calculation.EnergyDashboard(name, fc, year, m); err != nil {
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
