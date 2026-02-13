package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	protobuf "at.ourproject/energystore/protoc"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RequestActiveMeteringPoints(tenant string, from, to *uint64) ([]*protobuf.MeteringPoint, error) {
	conn, err := grpc.Dial(viper.GetString("services.master-server"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	defer conn.Close()
	c := protobuf.NewApiServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	request := &protobuf.MeteringRequest{
		Tenant: tenant,
		Status: 0,
		From:   from,
		To:     to,
	}

	r, err := c.MasterData_MeteringPoint(ctx, request)
	glog.V(5).Infof("Response from MASTER-DATA Service: %v", r)
	if r == nil {
		if err != nil {
			glog.Error(fmt.Sprintf("Error Servicecall: %v", err))
		}
		glog.Error(errors.New("error fetch Meteringpoints"))
		return nil, errors.New("error fetch Meteringpoints")
	}
	return r.MeteringPoints, err
}
