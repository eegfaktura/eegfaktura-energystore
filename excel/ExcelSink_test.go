package excel

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestExportExcel(t *testing.T) {
	viper.Set("persistence.path", "../../../rawdata")

	cps := &ExportCPs{
		CommunityId: "ATSEPPHUBER",
		Cps: []InvestigatorCP{
			{
				MeteringPoint: "AT0030000000000000000000000351391",
				Direction:     "CONSUMPTION",
				Name:          "Stefan Maier",
			},
			{
				MeteringPoint: "AT0030000000000000000000000379812",
				Direction:     "CONSUMPTION",
				Name:          "Michael Schauer",
			},
			{
				MeteringPoint: "AT0030000000000000000000030043080",
				Direction:     "GENERATION",
				Name:          "Michael Schauer",
			},
			{
				MeteringPoint: "AT0030000000000000000000000381701",
				Direction:     "CONSUMPTION",
				Name:          "Stefan Maier",
			},
			{
				MeteringPoint: "AT0030000000000000000000000655856",
				Direction:     "CONSUMPTION",
				Name:          "Helmut Schustereder",
			},
		},
	}

	//buf, err := ExportExcel("RC100181", 2023, 2, cps)

	start := time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, time.Month(4)+1, 0, 0, 0, 0, 0, time.UTC)
	buf, err := CreateExcelFile("RC100181", start, end, cps)

	//conn, err := grpc.Dial("127.0.0.1:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	//require.NoError(t, err)
	//defer conn.Close()
	//c := protobuf.NewExcelAdminServiceClient(conn)
	//
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	//defer cancel()

	filename := fmt.Sprintf("1-RC100181-Energie Report-%d%.2d.xlsx", 2023, 2)
	err = os.WriteFile(fmt.Sprintf("./%s", filename), buf.Bytes(), 0644)
	require.NoError(t, err)
	//r, err := c.SendExcel(ctx, &protobuf.SendExcelRequest{Tenant: "ADMIN", Recipient: "obermueller.peter@gmail.com", Filename: &filename, Content: buf.Bytes()})
	//require.NoError(t, err)
	//fmt.Printf("Response from GRPC: %+v\n", r)

	assert.NoError(t, err)
}
