package excel

import (
	protobuf "at.ourproject/energystore/protoc"
	"context"
	"fmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

func TestExportExcel(t *testing.T) {
	viper.Set("persistence.path", "../../../rawdata")
	buf, err := ExportExcel("RC100181", 2023, 3)
	conn, err := grpc.Dial("127.0.0.1:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	c := protobuf.NewExcelAdminServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	r, err := c.SendExcel(ctx, &protobuf.SendExcelRequest{Tenant: "ADMIN", Recipient: "obermueller.peter@gmail.com", Filename: "SEpp.xlsx", Content: buf.Bytes()})
	require.NoError(t, err)
	fmt.Printf("Response from GRPC: %+v\n", r)

	assert.NoError(t, err)
}
