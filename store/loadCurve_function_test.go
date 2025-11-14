package store

import (
	"fmt"
	"math"
	"testing"
	"time"

	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getSeptember() []*model.RawSourceLine {
	return []*model.RawSourceLine{
		{Id: "CP/2025/09/09/00/00/00", Consumers: []float64{4.01}, Producers: []float64{}},
		{Id: "CP/2025/09/10/00/00/00", Consumers: []float64{3.01}, Producers: []float64{}},
		{Id: "CP/2025/09/11/00/00/00", Consumers: []float64{2.01}, Producers: []float64{}},
		{Id: "CP/2025/09/12/00/00/00", Consumers: []float64{1.01}, Producers: []float64{}},
	}
}

func getOctober() []*model.RawSourceLine {
	return []*model.RawSourceLine{
		{Id: "CP/2025/10/09/00/00/00", Consumers: []float64{4.01}, Producers: []float64{}},
		{Id: "CP/2025/10/10/00/00/00", Consumers: []float64{3.01}, Producers: []float64{}},
		{Id: "CP/2025/10/11/00/00/00", Consumers: []float64{2.01}, Producers: []float64{}},
		{Id: "CP/2025/10/12/00/00/00", Consumers: []float64{1.01}, Producers: []float64{}},

		{Id: "CP/2025/10/13/00/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/00/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/00/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/00/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/13/01/00/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2025/10/13/01/15/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2025/10/13/01/30/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2025/10/13/01/45/00", Consumers: []float64{0.02}, Producers: []float64{}},

		{Id: "CP/2025/10/13/02/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/02/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/02/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/13/02/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/14/03/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/03/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/03/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/03/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/14/10/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/10/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/10/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/14/10/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/15/00/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/00/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/00/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/00/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/15/04/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/04/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/04/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/15/04/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/16/05/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/16/05/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/16/05/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/16/05/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/17/06/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/06/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/06/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/06/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/17/12/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/12/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/12/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/17/12/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/17/13/00/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2025/10/17/13/15/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2025/10/17/13/30/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2025/10/17/13/45/00", Consumers: []float64{0.1}, Producers: []float64{}},

		{Id: "CP/2025/10/17/14/00/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2025/10/17/14/15/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2025/10/17/14/30/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2025/10/17/14/45/00", Consumers: []float64{0.2}, Producers: []float64{}},

		{Id: "CP/2025/10/17/15/00/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2025/10/17/15/15/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2025/10/17/15/30/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2025/10/17/15/45/00", Consumers: []float64{0.3}, Producers: []float64{}},

		{Id: "CP/2025/10/18/00/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/00/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/00/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/00/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/18/07/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/07/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/07/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/07/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/18/14/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/14/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/14/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/18/14/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/15/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/15/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/15/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/15/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/08/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/08/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/08/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/08/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/09/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/09/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/09/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/09/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/16/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/16/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/16/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/16/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/17/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/17/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/17/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/17/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/18/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/18/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/18/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/18/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/19/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/19/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/19/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/19/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/20/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/20/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/20/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/20/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/21/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/21/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/21/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2025/10/19/21/45/00", Consumers: []float64{0.01}, Producers: []float64{}},

		{Id: "CP/2025/10/19/22/00/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2025/10/19/22/15/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2025/10/19/22/30/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2025/10/19/22/45/00", Consumers: []float64{0.05}, Producers: []float64{}},

		{Id: "CP/2025/10/19/23/00/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2025/10/19/23/15/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2025/10/19/23/30/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2025/10/19/23/45/00", Consumers: []float64{0.4}, Producers: []float64{}},
	}
}

func getDay() []*model.RawSourceLine {
	return []*model.RawSourceLine{
		{Id: "CP/2022/11/08/00/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/00/00", Consumers: []float64{0.02, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/15/00", Consumers: []float64{0.02, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/30/00", Consumers: []float64{0.02, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/45/00", Consumers: []float64{0.02, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/00/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/15/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/30/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/45/00", Consumers: []float64{0.01, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/00/00", Consumers: []float64{0.05, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/15/00", Consumers: []float64{0.05, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/30/00", Consumers: []float64{0.05, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/45/00", Consumers: []float64{0.05, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/00/00", Consumers: []float64{0.1, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/15/00", Consumers: []float64{0.1, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/30/00", Consumers: []float64{0.1, 0.0, 0.0}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/45/00", Consumers: []float64{0.1, 0.0, 0.0}, Producers: []float64{}},
	}
}

func TestLoadCurve_Function(t *testing.T) {

	type args struct {
		start  time.Time
		end    time.Time
		tsFn   AddCacheTimeFunc
		initFn InitCacheTimeFunc
		nameFn SeriesNameFunc
		cps    []TargetMP
		lines  []*model.RawSourceLine
	}
	tests := []struct {
		name string
		args args
		want func(t *testing.T, result []interface{})
	}{
		{
			name: "Aggregate Years",
			args: args{
				start: time.Date(2025, time.Month(9), 9, 0, 0, 0, 0, time.Local),
				end:   time.Date(2025, 10, 19, 0, 0, 0, 0, time.Local),
				tsFn:  AddDate(0, 1, 0), initFn: InitMonth(), nameFn: monthYearNameFunc(),
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: append(getSeptember(), getOctober()...),
			},
			want: func(t *testing.T, result []interface{}) {
				assert.Equal(t, 2, len(result))
				data := result[0].(*ReportNamedData)
				assert.Equal(t, 10.04, math.Round(data.ReportData.Consumed*100)/100)
				assert.Equal(t, "M:2025:09:09", data.Name)
				data = result[1].(*ReportNamedData)
				assert.Equal(t, 15.16, math.Round(data.ReportData.Consumed*100)/100)
				assert.Equal(t, "M:2025:10:10", data.Name)
			},
		},
		{
			name: "Aggregate Days",
			args: args{
				start: time.Date(2025, time.Month(10), 9, 0, 0, 0, 0, time.Local),
				end:   time.Date(2025, 10, 19, 0, 0, 0, 0, time.Local),
				tsFn:  AddDate(0, 0, 1), nameFn: monthDayNameFunc(), initFn: InitDefault(),
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getOctober(),
			},
			want: func(t *testing.T, result []interface{}) {
				assert.Equal(t, 11, len(result))
				data := result[0].(*ReportNamedData)
				assert.Equal(t, 4.01, math.Round(data.ReportData.Consumed*100)/100)
				assert.Equal(t, "D:10:09:03", data.Name)
				data = result[1].(*ReportNamedData)
				assert.Equal(t, 3.01, math.Round(data.ReportData.Consumed*100)/100)
				assert.Equal(t, "D:10:10:04", data.Name)
				data = result[10].(*ReportNamedData)
				assert.Equal(t, 2.16, math.Round(data.ReportData.Consumed*100)/100)
				assert.Equal(t, "D:10:19:06", data.Name)
			},
		},
		{
			name: "Aggregate Week",
			args: args{
				start: time.Date(2025, time.Month(10), 9, 0, 0, 0, 0, time.Local),
				end:   time.Date(2025, 10, 19, 0, 0, 0, 0, time.Local),
				tsFn:  AddDate(0, 0, 7), nameFn: weekYearNameFunc(), initFn: InitWeek(),
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getOctober(),
			},
			want: func(t *testing.T, result []interface{}) {
				assert.Equal(t, 2, len(result))
				data := result[1].(*ReportNamedData).ReportData
				assert.Equal(t, 5.12, math.Round(data.Consumed*100)/100)
				assert.Equal(t, 0.00, data.Allocated)
				assert.Equal(t, 0.00, data.Distributed)
				assert.Equal(t, 0.00, data.Produced)
				assert.Equal(t, 0.00, data.Unused)
			},
		},
		{
			name: "Aggregate Raw",
			args: args{
				start: time.Date(2025, time.Month(11), 8, 0, 0, 0, 0, time.Local),
				end:   time.Date(2025, 11, 8, 23, 59, 59, 0, time.Local),
				tsFn:  nil, nameFn: dayRawNameFunc(), initFn: InitDefault(),
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getDay(),
			},
			want: func(t *testing.T, result []interface{}) {
				//assert.Equal(t, 2, len(result))
				//data := result[1].(*ReportNamedData).ReportData
				//assert.Equal(t, 5.12, math.Round(data.Consumed*100)/100)
				//assert.Equal(t, 0.00, data.Allocated)
				//assert.Equal(t, 0.00, data.Distributed)
				//assert.Equal(t, 0.00, data.Produced)
				//assert.Equal(t, 0.00, data.Unused)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := createTestEngineContext(tt.args.start, tt.args.end)
			require.NoError(t, err)

			agg, err := NewLoadCurveFunction(tt.args.tsFn, tt.args.nameFn, tt.args.initFn)
			require.NoError(t, agg.HandleStart(ctx))
			for _, l := range tt.args.lines {
				require.NoError(t, agg.HandleLine(ctx, l))
			}

			require.NoError(t, agg.HandleEnd(ctx))

			if tt.want != nil {
				tt.want(t, (agg.(EnergyReportConsumer)).GetResult())
			}
			r := (agg).(EnergyReportConsumer).GetResult()
			for _, rr := range r {
				fmt.Printf("%s: %+v\n", rr.(*ReportNamedData).Name, rr.(*ReportNamedData).ReportData)
			}
		})
	}
}
