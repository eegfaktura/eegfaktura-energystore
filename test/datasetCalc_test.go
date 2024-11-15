package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eegfaktura/eegfaktura-energystore/calculation"
	"github.com/eegfaktura/eegfaktura-energystore/model"
	"github.com/eegfaktura/eegfaktura-energystore/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateReports(t *testing.T) {

	db, err := store.OpenStorageTest("excelsource", "ecIdTest", "../test/rawdata")
	require.NoError(t, err)
	defer func() {
		db.Close()
		//os.RemoveAll("../test/rawdata/excelsource")
	}()

	ImportTestContent(t, "./zaehlpunkte-beispieldatei.xlsx", "ConsumptionDataReport", db)
	//for _, k := range ImportTestContent(t, "./zaehlpunkte-beispieldatei.xlsx", "ConsumptionDataReport", db) {
	//	err = calculation.CalculateMonthlyDash(db, fmt.Sprintf("%d", k), calculation.CalculateEEG)
	//	require.NoError(t, err)
	//}

	t.Run("Calc yearly report", func(t *testing.T) {
		//results, report, err := calculation.CalculateYearlyReport(db, 2021, calculation.CalculateEEG)
		timeStart := time.Now()
		results, report, err := calculation.CalculateAnnualPeriod(db, calculation.AllocDynamicV2, 2021)
		require.NoError(t, err)
		require.NotNil(t, results)
		require.NotNil(t, report)
		require.NotNil(t, report.Allocated)

		for _, r := range results {
			fmt.Printf("Allocated: %+v\n", r.Allocated)
			fmt.Printf("Consumed: %+v\n", r.Consumed)
			fmt.Printf("Produced: %+v\n", r.Produced)
			fmt.Printf("Shared: %+v\n", r.Shared)
			fmt.Printf("TotalProduced: %+v\n", r.TotalProduced)
			fmt.Println("----------------------")
			fmt.Printf("%+v\n", r)
			fmt.Println("----------------------")
		}

		expectedResult := []*model.EnergyReport{
			{Id: "IRP/2021/0/01/", Allocated: []float64{32.554337, 15.27957, 28.427826, 21.769701, 22.619209, 20.632025, 18.976233, 8.930697, 26.378684, 17.069989, 12.113755, 16.261013, 5.62212, 13.580913, 19.522484, 6.540281, 8.722579, 14.733491, 5.16557, 9.688355, 6.479069, 20.850963, 9.517167, 5.858366, 2.568325, 7.371698, 18.200961, 16.508037, 1.502406, 14.204384, 29.245845, 15.403199}, Consumed: []float64{599.2975, 216.22825, 426.85675, 467.17, 315.331, 190.87775, 246.98225, 162.88625, 315.9515, 368.9225, 247.952, 231.194, 88.12875, 191.16825, 214.1225, 130.34925, 164.704, 290.97025, 68.936, 142.5025, 101.8015, 291.0505, 123.6015, 104.0345, 64.517, 150.15575, 280.15425, 330.62975, 32.196, 226.9435, 442.1945, 257.0915}, Produced: []float64{475.05}, Distributed: []float64{472.29925}, Shared: []float64{32.822989, 15.401108, 28.516484, 21.903876, 22.717238, 20.80553, 19.059038, 8.965409, 26.587028, 17.186867, 12.192635, 16.394158, 5.684071, 13.611747, 19.653363, 6.602797, 8.767936, 14.847559, 5.210471, 9.728867, 6.534805, 20.916124, 9.55294, 5.879, 2.59882, 7.416362, 18.255336, 16.558864, 1.513291, 14.300504, 29.388391, 15.476389}, TotalProduced: 475.0499999999992},
			{Id: "IRP/2021/0/02/", Allocated: []float64{187.149833, 85.561138, 109.799873, 121.177558, 92.386176, 63.039483, 82.266492, 38.554523, 79.845547, 94.216094, 75.101495, 61.47784, 28.062522, 50.996219, 95.612174, 36.983769, 39.263938, 57.707211, 23.783592, 31.260605, 24.84084, 97.538348, 39.262102, 32.691348, 16.129771, 47.074481, 97.924603, 68.91258, 8.066615, 64.524486, 142.763576, 83.814919}, Consumed: []float64{554.43325, 205.97025, 282.42775, 420.13275, 248.87725, 155.455, 187.325, 119.81575, 235.60225, 351.05075, 204.409, 199.776, 80.87775, 116.855, 204.251, 125.142, 132.418, 206.368, 57.71125, 103.53075, 87.13925, 241.0115, 100.1955, 89.8445, 57.10825, 145.2395, 244.46525, 219.933, 26.9815, 214.12775, 369.394, 232.70125}, Produced: []float64{2448.5895}, Distributed: []float64{2177.78975}, Shared: []float64{215.81931, 96.552307, 122.543275, 137.527881, 104.382532, 69.34833, 93.125663, 42.354166, 89.384409, 106.784952, 85.815648, 67.376801, 31.009481, 56.582562, 109.162896, 41.608069, 46.020769, 63.546158, 26.196266, 35.399352, 27.842102, 107.306172, 44.269168, 36.261648, 18.460931, 53.754575, 108.371524, 75.479593, 9.219388, 70.195889, 161.70285, 95.184833}, TotalProduced: 2448.589500000004},
			{Id: "IRP/2021/0/03/", Allocated: []float64{227.345241, 106.696112, 141.486914, 134.831977, 103.220096, 103.847302, 111.329055, 57.722858, 105.593906, 78.870947, 95.316202, 102.957465, 27.456456, 122.205107, 105.680025, 42.984035, 42.898537, 83.60773, 25.168898, 31.923308, 29.378355, 148.877727, 44.526107, 37.941586, 21.772865, 39.683917, 115.910645, 80.82233, 10.667009, 48.646419, 160.916535, 85.312083}, Consumed: []float64{619.158, 238.0815, 306.74225, 424.8785, 271.82925, 211.727, 261.44325, 144.96525, 262.0885, 294.571, 233.40175, 256.38, 81.522, 303.73125, 223.14425, 134.9295, 106.663, 268.71475, 60.903, 101.3505, 89.72075, 312.01675, 109.37675, 111.312, 71.93725, 123.27475, 270.322, 249.37725, 34.3045, 163.11325, 389, 227.21525}, Produced: []float64{3623.469}, Distributed: []float64{2675.59775}, Shared: []float64{323.85977, 149.48401, 190.499102, 187.690589, 139.972761, 142.505586, 153.18073, 77.99603, 153.443568, 103.940271, 126.675489, 129.291203, 36.115306, 164.404821, 152.177066, 57.424614, 56.642065, 113.631285, 33.724964, 38.122363, 37.595262, 187.014001, 60.253379, 49.010096, 27.914747, 54.863701, 156.047776, 106.036287, 14.734549, 61.888239, 221.0851, 116.24427}, TotalProduced: 3623.468999999997},
			{Id: "IRP/2021/0/04/", Allocated: []float64{288.640973, 162.915139, 170.991695, 188.483301, 136.844154, 118.814849, 140.237108, 72.074497, 326.778811, 118.625272, 124.116714, 126.133503, 39.352521, 80.118466, 152.69946, 74.885015, 50.591681, 97.107264, 33.845247, 57.094079, 38.369089, 111.592818, 62.588668, 51.3626, 32.396258, 83.312764, 150.103285, 107.288227, 15.043054, 41.192565, 209.219632, 123.51004}, Consumed: []float64{565.08675, 253.51, 332.48925, 400.26625, 259.199, 193.57525, 246.792, 126.34625, 446.93725, 284.5105, 228.69875, 222.836, 76.84925, 125.644, 223.3175, 159.86125, 96.631, 214.037, 58.40475, 98.26275, 77.834, 183.54325, 99.67825, 97.7255, 65.80775, 156.72075, 248.05975, 211.10025, 30.592, 102.40125, 375.77875, 237.28875}, Produced: []float64{5573.8935}, Distributed: []float64{3586.32875}, Shared: []float64{469.375041, 269.163441, 247.799128, 285.908999, 195.68134, 181.177056, 231.942331, 118.810256, 568.348038, 177.264982, 195.150707, 184.495186, 55.304036, 120.372459, 248.922696, 114.974952, 77.473191, 140.545754, 52.937222, 87.307003, 57.269518, 156.595806, 101.534093, 71.696691, 48.243834, 136.298752, 218.919291, 154.699224, 22.709488, 63.059401, 326.427893, 193.485691}, TotalProduced: 5573.893500000005},
			{Id: "IRP/2021/0/05/", Allocated: []float64{319.161081, 202.18418, 207.165596, 223.508193, 162.315989, 145.363783, 98.554665, 75.348868, 408.920777, 147.219585, 132.641648, 145.715244, 47.701127, 120.278633, 140.833423, 87.588137, 56.041638, 155.240098, 49.812892, 71.743908, 43.972129, 129.24304, 66.394232, 55.034556, 39.346277, 90.207457, 202.833095, 157.220482, 16.614666, 135.806217, 253.902727, 116.980659}, Consumed: []float64{558.28525, 275.9605, 308.92825, 423.0105, 261.83375, 205.71025, 167.78225, 121.51975, 525.612, 303.609, 215.374, 217.261, 80.0085, 263.25825, 192.40325, 142.3205, 102.236, 273.126, 68.6595, 111.943, 75.864, 182.28525, 94.89875, 89.56025, 63.14325, 152.9105, 283.70225, 235.0375, 30.6855, 229.21675, 374.1695, 188.7195}, Produced: []float64{6701.934}, Distributed: []float64{4304.895}, Shared: []float64{520.907472, 330.149004, 327.891698, 336.493661, 239.23437, 222.424001, 155.298877, 125.221376, 638.308294, 219.15859, 216.030517, 213.781658, 69.667539, 177.74196, 254.053022, 134.499044, 91.897601, 243.117442, 76.573288, 102.092977, 63.880082, 200.051175, 103.28564, 80.146205, 59.107659, 139.559549, 302.821635, 234.819658, 26.383024, 210.778225, 399.007039, 187.55172}, TotalProduced: 6701.9340000000175},
			{Id: "IRP/2021/0/06/", Allocated: []float64{334.024455, 227.858301, 203.179574, 111.335282, 137.583026, 106.841666, 130.845272, 63.449344, 298.676722, 148.718163, 123.145325, 83.916328, 57.494292, 120.950759, 167.244099, 76.384374, 72.235974, 81.691184, 51.894605, 56.418391, 47.624812, 150.964508, 88.907939, 63.215397, 35.08198, 89.269587, 206.072887, 91.455762, 22.365188, 176.366259, 155.636478, 101.670817}, Consumed: []float64{475.88, 268.88775, 258.5755, 179.2905, 191.01875, 135.62225, 180.0245, 87.61375, 350.3675, 230.657, 168.6355, 115.441, 80.14075, 283.49525, 201.6945, 113.57725, 102.491, 116.995, 63.9065, 76.25325, 66.423, 202.37925, 107.7405, 87.4115, 52.1595, 131.621, 248.43625, 112.50125, 35.501, 253.59725, 207.44575, 132.69275}, Produced: []float64{9253.6845}, Distributed: []float64{3882.51875}, Shared: []float64{870.675209, 575.626106, 472.044559, 258.584436, 325.53872, 249.964399, 310.975721, 153.644641, 785.224999, 320.400162, 281.501968, 201.96521, 121.570091, 254.400287, 459.784596, 181.634382, 188.398733, 195.897445, 130.975859, 119.834038, 101.686535, 338.215745, 209.528038, 144.379938, 87.601537, 200.747942, 456.151463, 180.412021, 54.570163, 395.540311, 374.39522, 251.814028}, TotalProduced: 9253.684500000005},
			{Id: "IRP/2021/0/07/", Allocated: []float64{328.907463, 226.208981, 206.692875, 103.708826, 122.65549, 142.907421, 63.05535, 70.43088, 292.470734, 151.733528, 123.751406, 145.149972, 48.426283, 71.005353, 153.348262, 85.168111, 79.630288, 58.608199, 44.734589, 50.005804, 48.452807, 136.617379, 82.044863, 49.036094, 35.824953, 77.265018, 187.962554, 81.77125, 20.961402, 183.057864, 158.816717, 84.219534}, Consumed: []float64{514.9005, 287.61275, 284.4165, 184.906, 205.9505, 185.6595, 102.072, 100.8145, 370.1395, 280.703, 181.42225, 200.882, 74.10825, 97.42175, 195.49875, 133.67375, 121.969, 95.557, 59.562, 71.19275, 72.56525, 191.254, 105.76075, 77.50675, 52.43525, 130.8665, 254.9115, 111.64275, 35.85525, 263.69725, 251.3025, 129.1885}, Produced: []float64{7691.601}, Distributed: []float64{3714.63025}, Shared: []float64{738.993534, 488.95987, 421.794299, 210.18152, 258.353583, 281.394577, 125.686114, 136.658854, 667.44256, 287.510998, 268.450407, 295.040646, 95.650024, 162.981921, 355.420516, 177.260917, 163.164087, 111.393046, 96.640951, 94.783905, 94.552814, 267.280894, 185.240516, 102.606579, 80.1995, 149.508871, 371.47369, 143.405763, 45.172607, 363.798825, 278.532396, 172.066215}, TotalProduced: 7691.601000000016},
			{Id: "IRP/2021/0/08/", Allocated: []float64{272.524249, 189.172187, 221.404372, 0.40225, 117.064438, 144.188321, 61.472778, 79.365223, 274.497786, 121.53128, 121.980786, 133.637256, 48.172896, 107.297504, 164.996672, 71.442577, 59.26778, 54.659557, 44.346646, 54.768377, 39.754686, 130.996049, 74.397949, 53.506401, 27.583705, 94.337451, 172.786455, 85.646414, 18.277763, 156.009375, 203.322485, 79.716911}, Consumed: []float64{487.682, 270.86225, 339.69225, 0.993984, 205.1235, 200.1975, 109.50925, 124.34875, 374.51925, 270.44525, 189.75375, 197.062, 81.93875, 195.884, 228.9335, 134.81625, 108.191, 97.74725, 64.4825, 85.02075, 70.919, 203.51375, 106.5305, 92.8685, 49.59, 154.5355, 260.912, 147.321, 34.941, 251.51075, 309.45325, 130.64025}, Produced: []float64{5983.2225}, Distributed: []float64{3478.52858}, Shared: []float64{506.61721, 336.176259, 382.447773, 0.423518, 187.884919, 226.198992, 116.707279, 130.333919, 502.117489, 202.62096, 206.878127, 227.011625, 80.451812, 184.671274, 299.545675, 119.527826, 100.088562, 87.788869, 75.550201, 104.286116, 69.259166, 192.761109, 131.206001, 95.116468, 47.270112, 171.484449, 288.898194, 131.468281, 32.919113, 278.198914, 331.704935, 135.607352}, TotalProduced: 5983.222499999999},
			{Id: "IRP/2021/0/09/", Allocated: []float64{270.726792, 186.758742, 173.242145, 0, 105.533032, 135.955239, 96.217525, 43.198935, 226.18689, 129.450301, 87.441162, 120.246112, 47.715493, 67.87573, 109.11914, 64.320856, 55.333325, 60.515881, 40.719896, 41.976395, 38.573853, 91.747009, 65.135563, 46.104953, 29.376236, 82.533325, 155.258036, 114.697096, 15.786941, 145.95474, 199.54176, 70.101816}, Consumed: []float64{504.690784, 275.208718, 269.615024, 0, 197.799746, 183.244968, 170.28, 76.845502, 313.242468, 287.014042, 155.292774, 204.391016, 83.495252, 155.13326, 169.346984, 125.018468, 103.797016, 117.727218, 59.391546, 60.940016, 71.795996, 152.381498, 99.023724, 83.771262, 57.397718, 137.415966, 246.538984, 189.447756, 31.52574, 250.355716, 312.967986, 122.684986}, Produced: []float64{5518.587}, Distributed: []float64{3117.344921}, Shared: []float64{505.675169, 340.209333, 339.452297, 0, 182.466575, 232.359019, 154.041937, 73.874592, 417.705491, 221.777999, 157.09033, 207.838977, 82.634318, 123.362401, 196.074989, 113.474727, 96.138863, 105.330763, 68.221387, 83.649474, 62.838755, 150.65847, 119.198543, 81.154054, 51.727587, 162.151338, 256.11042, 200.376477, 29.094803, 246.057268, 342.849031, 114.99161}, TotalProduced: 5518.587000000004},
			{Id: "IRP/2021/0/10/", Allocated: []float64{225.550532, 146.496575, 150.49923, 0, 91.666651, 76.141263, 87.604883, 38.921702, 121.227179, 98.123951, 101.792118, 103.21101, 28.517095, 137.501229, 65.703327, 42.014374, 37.99018, 70.687576, 27.045678, 28.85314, 23.779268, 54.151793, 55.534467, 31.765512, 31.553066, 60.234804, 102.693079, 95.983664, 9.837837, 106.286568, 171.859185, 105.008064}, Consumed: []float64{602.023, 301.671, 330.4655, 0, 231.31025, 162.98825, 205.00175, 103.663, 269.65275, 313.37475, 242.3875, 254.411, 88.295, 246.0645, 152.1605, 124.7455, 117.454, 215.612, 57.60425, 55.87225, 80.195, 151.105, 114.26725, 87.1235, 86.4865, 126.3055, 257.261, 265.03425, 29.662, 284.52875, 390.067, 278.8065}, Produced: []float64{3074.658}, Distributed: []float64{2528.235}, Shared: []float64{284.162878, 183.333875, 183.244883, 0, 109.743059, 90.218864, 106.322357, 47.104367, 146.130434, 119.814411, 122.189872, 121.583968, 34.024817, 162.153731, 78.882575, 51.130917, 50.185575, 87.233583, 31.826893, 33.881938, 27.915914, 65.722873, 66.238772, 39.388417, 38.244989, 75.676341, 124.901227, 114.285975, 12.39402, 127.649099, 212.066315, 127.005062}, TotalProduced: 3074.6579999999994},
			{Id: "IRP/2021/0/11/", Allocated: []float64{101.836351, 56.658606, 52.96779, 0, 37.82714, 44.984981, 48.41736, 20.054921, 51.393754, 41.981512, 47.624272, 38.073405, 13.357775, 41.971928, 36.563864, 19.10946, 20.498307, 38.861281, 12.052182, 9.854378, 13.767556, 30.797824, 25.480372, 14.323465, 18.37359, 28.517493, 52.380459, 41.261447, 4.242736, 50.494365, 86.256315, 59.315111}, Consumed: []float64{586.91425, 269.68625, 288.69375, 0, 242.015, 158.32225, 187.9105, 140.553, 284.45175, 332.29675, 239.9375, 252.794, 86.56975, 322.95625, 162.03825, 129.08875, 126.7, 272.33175, 60.23775, 60.63075, 83.7555, 181.16625, 114.1335, 97.75125, 93.90075, 133.2135, 289.641, 295.6195, 28.099, 318.4155, 423.39075, 311.20675}, Produced: []float64{1229.922}, Distributed: []float64{1159.3}, Shared: []float64{108.735214, 60.241217, 56.002697, 0, 40.348668, 48.059143, 51.976204, 21.177829, 54.617452, 44.479217, 50.765916, 40.442811, 14.136614, 44.032668, 38.473109, 20.224751, 21.990329, 41.391824, 12.791319, 10.447824, 14.478581, 32.647006, 27.301743, 15.023118, 19.202591, 30.02536, 55.183566, 43.489592, 4.509152, 53.040517, 91.735081, 62.95089}, TotalProduced: 1229.9220000000014},
			{Id: "IRP/2021/0/12/", Allocated: []float64{63.792035, 37.153532, 37.486996, 0, 24.622093, 26.728306, 34.446581, 17.291622, 31.262633, 27.659934, 29.483876, 23.960434, 8.713383, 19.558747, 29.912957, 17.884059, 15.155038, 27.250585, 9.263661, 7.194368, 9.077858, 21.106705, 15.980972, 9.874435, 7.131537, 13.788563, 40.337621, 27.777806, 2.777578, 26.300528, 53.439501, 37.533305}, Consumed: []float64{624.87675, 282.89375, 379.55225, 0, 275.94425, 160.47175, 293.18975, 165.525, 290.58925, 378.678, 265.0545, 241.569, 83.3295, 250.90775, 182.36575, 158.632, 150.165, 311.08225, 64.337, 64.118, 83.91075, 198.142, 126.57625, 109.85, 65.03, 115.46425, 312.6135, 272.75425, 31.80275, 302.93625, 419.5375, 314.3005}, Produced: []float64{772.8495}, Distributed: []float64{753.94725}, Shared: []float64{65.754454, 38.241567, 38.180748, 0, 25.149046, 27.417837, 35.234879, 17.464938, 31.904902, 28.408818, 30.172208, 24.740145, 8.950471, 19.933243, 30.763198, 18.283511, 15.445792, 27.888675, 9.493543, 7.338452, 9.273102, 21.621574, 16.311059, 10.105405, 7.237264, 14.132932, 41.683382, 28.408964, 2.851824, 26.967273, 54.902345, 38.587949}, TotalProduced: 772.8494999999992},
		}

		assert.Equal(t, results, expectedResult)
		fmt.Printf("Duration Annual Report %f\n", time.Since(timeStart).Seconds())
	})

	t.Run("calculate weekly report", func(t *testing.T) {
		begin := time.Now()
		//results, report, err := calculation.CalculateWeeklyReport(db, 2021, 4, calculation.CalculateEEG)
		results, report, err := calculation.CalculateMonthlyPeriod(db, calculation.AllocDynamicV2, 2021, 4)
		require.NoError(t, err)

		require.NotNil(t, results)
		require.NotNil(t, report)
		require.NotNil(t, report.Allocated)

		//fmt.Printf("Results: %+v\n", results)
		//fmt.Printf("Report: %+v\n", report)
		//
		//fmt.Println("Results:")
		//for _, r := range results {
		//	fmt.Printf("%+v\n", r)
		//}
		//fmt.Println("---------")

		expectedResult := []*model.EnergyReport{
			{Id: "IRP/2021/04/01",
				Allocated: []float64{10.694395, 6.216645, 5.500512, 6.754333, 3.186536, 8.134405, 5.065443, 1.367591, 6.667514, 4.210794,
					6.428909, 7.202596, 1.390592, 2.287389, 9.335845, 1.847196, 0.987931, 3.25504, 0.60375, 3.48169, 2.361264, 4.134785,
					3.188447, 2.62492, 0.654981, 1.899608, 6.610886, 2.189172, 0.520364, 1.675058, 10.587324, 4.653335},
				Consumed: []float64{18.28675, 8.58, 8.822, 13.4815, 5.68225, 9.25325, 7.6875, 4.316, 9.143, 7.53075, 9.2845,
					8.872, 2.204, 3.58875, 11.1715, 4.26875, 2.203, 6.50975, 1.13725, 4.54875, 3.619, 5.79125, 4.42725, 5.0385,
					1.2585, 3.777, 8.8805, 4.02075, 0.99025, 5.01325, 12.86375, 8.305},
				Produced:    []float64{238.9545},
				Distributed: []float64{135.71925},
				Shared: []float64{19.467661, 11.572262, 8.579713, 11.834236, 5.517892, 15.239768, 10.954444, 2.301589,
					12.829686, 6.615176, 13.919331, 11.126067, 1.981088, 3.432402, 18.172201, 2.955645, 1.756052,
					5.781507, 1.057842, 6.931364, 3.77537, 7.266874, 5.434402, 4.212624, 1.174374, 2.750119, 9.30025,
					3.605523, 0.920976, 3.001361, 18.06324, 7.42346},
				//Allocated:     []float64{10.694395118167739, 6.216645031801386, 5.500512025405309, 6.754333198657755, 3.1865359660168986, 8.134404641303474, 5.06544268744124, 1.367590883777918, 6.667514077293756, 4.2107942218018435, 6.428908861255079, 7.202596449766271, 1.390592024264578, 2.287389121647259, 9.335844724004286, 1.847196251634725, 0.9879306591537823, 3.2550395019073024, 0.6037496836755272, 3.4816898564398753, 2.3612636546856907, 4.1347849135276, 3.188446955496431, 2.624920244018871, 0.6549812477940722, 1.8996083884004669, 6.6108855743159705, 2.189172133105621, 0.5203641418134429, 1.6750583477122807, 10.587324045325799, 4.65333536838775},
				//Consumed:      []float64{18.286749999999998, 8.58, 8.822000000000005, 13.481500000000006, 5.6822500000000025, 9.253249999999998, 7.6875, 4.316000000000001, 9.142999999999999, 7.530749999999998, 9.2845, 8.871999999999996, 2.2040000000000006, 3.5887499999999997, 11.171499999999996, 4.268750000000001, 2.2029999999999994, 6.509750000000002, 1.1372499999999997, 4.548749999999998, 3.6189999999999984, 5.791249999999995, 4.427250000000001, 5.038499999999998, 1.2584999999999995, 3.7770000000000015, 8.880499999999998, 4.02075, 0.9902499999999999, 5.01325, 12.863749999999996, 8.305000000000001},
				//Shared:        []float64{19.46766085482371, 11.572261827539888, 8.579712597308973, 11.834236256065651, 5.517892338695541, 15.239768151545832, 10.954443927882695, 2.301588561468027, 12.82968553997718, 6.615176362535689, 13.919330980172663, 11.126067406805321, 1.9810884454433666, 3.432401995159349, 18.172201024963822, 2.955644991189301, 1.756052167713177, 5.781506765875958, 1.0578424993529794, 6.931364289197004, 3.775369939281381, 7.266874405748571, 5.434401838857994, 4.212623583300323, 1.174373724544554, 2.7501189950375666, 9.300250192527638, 3.6055233519891265, 0.9209762946583417, 3.001360809641592, 18.063239500671383, 7.423460380025395},
				//Distributed:   []float64{135.71925000000002},
				//Produced:      []float64{238.9545},
				TotalProduced: 238.9545,
			},
			{Id: "IRP/2021/04/02",
				Allocated: []float64{11.843366, 5.601556, 4.457803, 8.029377, 3.181238, 2.64302, 5.145526, 1.590652, 5.505238, 3.594992,
					4.083432, 4.853377, 2.168954, 3.717319, 8.735379, 3.12658, 3.399975, 3.97525, 0.825517, 2.419149, 3.888236, 4.155166,
					1.59679, 2.856261, 0.677212, 2.75211, 6.070667, 4.975951, 0.527643, 1.71308, 5.683431, 6.678004},
				Consumed: []float64{19.05775, 7.76375, 6.44875, 15.30925, 5.459, 3.78675, 8.069, 2.64725, 7.99375, 6.63375,
					7.211, 6.901, 3.08175, 4.75625, 10.1375, 4.63425, 4.654, 8.0165, 1.328, 2.93425, 4.8615, 5.50725,
					2.447, 3.85475, 1.2645, 4.88425, 7.61725, 7.7895, 0.9755, 3.15575, 9.301, 10.1375},
				Produced:    []float64{156.0225},
				Distributed: []float64{130.47225},
				Shared: []float64{14.715637, 6.892607, 5.25611, 9.855895, 3.761223, 2.887163, 6.074558, 1.962988, 6.304212, 4.308116,
					4.616398, 5.857776, 2.532482, 4.532348, 9.744838, 3.780801, 4.016956, 4.698634, 0.934681, 3.682099, 4.832547,
					5.341361, 1.844377, 3.313911, 0.817873, 3.444055, 7.509814, 5.626806, 0.626062, 2.073515, 6.599488, 7.577166},
				//Allocated:     []float64{11.843365752841647, 5.6015560833992, 4.457802727170282, 8.029377158431263, 3.181238146853345, 2.643019700614921, 5.145526140327518, 1.5906521434695575, 5.505238260000745, 3.5949924349427413, 4.083431978964614, 4.853376664203659, 2.168953528321724, 3.7173190314109306, 8.735379019135028, 3.126580250373606, 3.399975058611172, 3.9752496990649, 0.8255168414606533, 2.419148870704692, 3.8882361163735863, 4.155165864627362, 1.596790425886767, 2.856260555852949, 0.6772123571041471, 2.7521095824216197, 6.070667464737086, 4.975950647674643, 0.5276426011418466, 1.7130795291806118, 5.683431069876102, 6.678004294821084},
				//Consumed:      []float64{19.057750000000002, 7.76375, 6.4487499999999995, 15.309249999999995, 5.459, 3.7867500000000023, 8.069, 2.6472500000000005, 7.993750000000001, 6.633749999999998, 7.2109999999999985, 6.901000000000002, 3.081750000000001, 4.756250000000002, 10.137500000000001, 4.63425, 4.653999999999998, 8.016499999999999, 1.3279999999999996, 2.9342500000000005, 4.861500000000001, 5.507249999999999, 2.4470000000000005, 3.8547500000000015, 1.2645000000000002, 4.884250000000001, 7.61725, 7.789500000000001, 0.9754999999999997, 3.1557500000000007, 9.300999999999998, 10.137500000000001},
				//Shared:        []float64{14.71563729144611, 6.892607249993103, 5.256110050174707, 9.85589535158196, 3.761223471590361, 2.887162760373918, 6.074557675044612, 1.9629882232278806, 6.30421224257407, 4.308116389832817, 4.616397894588586, 5.857776159286179, 2.532482349098504, 4.532348261517978, 9.744837858716826, 3.78080145337449, 4.016955663332867, 4.69863391973811, 0.9346807101201694, 3.6820992524606706, 4.832547435767749, 5.341360935176702, 1.8443765938845411, 3.3139114255161433, 0.8178731125543971, 3.4440554552929066, 7.509814298081315, 5.626805610966223, 0.6260617706334006, 2.0735151598042676, 6.599488029515099, 7.5771659447333395},
				//Distributed:   []float64{130.47224999999997},
				//Produced:      []float64{156.02250000000004},
				TotalProduced: 156.02250000000004,
			},
		}
		expectedReport := &model.EnergyReport{
			Id: "YM/2021/04",
			Allocated: []float64{288.640976, 162.915139, 170.991695, 188.483301, 136.844154, 118.81485, 140.237108, 72.074496,
				326.77881, 118.625271, 124.116715, 126.133501, 39.35252, 80.118468, 152.699459, 74.885015, 50.591682,
				97.107267, 33.845247, 57.09408, 38.36909, 111.592817, 62.588667, 51.362599, 32.396256, 83.312766,
				150.103284, 107.288226, 15.043054, 41.192565, 209.21963, 123.510039},
			Consumed: []float64{565.08675, 253.51, 332.48925, 400.26625, 259.199, 193.57525, 246.792, 126.34625, 446.93725,
				284.5105, 228.69875, 222.836, 76.84925, 125.644, 223.3175, 159.86125, 96.631, 214.037, 58.40475, 98.26275,
				77.834, 183.54325, 99.67825, 97.7255, 65.80775, 156.72075, 248.05975, 211.10025, 30.592, 102.40125, 375.77875,
				237.28875},
			Produced:    []float64{5573.8935},
			Distributed: []float64{3586.32875},
			Shared: []float64{469.375039, 269.163441, 247.799129, 285.908997, 195.68134, 181.177057, 231.942332,
				118.810255, 568.348039, 177.26498, 195.150708, 184.495185, 55.304036, 120.37246, 248.922694,
				114.974952, 77.473192, 140.545753, 52.937221, 87.307004, 57.269519, 156.595802, 101.534091,
				71.696688, 48.243834, 136.29875, 218.919291, 154.699225, 22.709488, 63.059401, 326.427894, 193.485689}, //Allocated: []float64{
			//	288.6409728407094, 162.9151388722588, 170.99169476050096, 188.483301092259,
			//	136.84415399311493, 118.81484945130057, 140.23710837627564, 72.07449682150494,
			//	326.77881103673946, 118.62527215331247, 124.11671386651602, 126.13350316934512,
			//	39.352521388214114, 80.11846619080164, 152.6994597802239, 74.88501499719344,
			//	50.59168077547026, 97.10726431948108, 33.84524683056678, 57.09407900498315, 38.36908897473679,
			//	111.59281839674811, 62.58866793011516, 51.362599688458566, 32.3962579350741, 83.31276432436539,
			//	150.10328547707215, 107.28822708419175, 15.043053893632024, 41.19256463691594, 209.21963230829328,
			//	123.51003962962506},
			//Consumed: []float64{
			//	565.0867499999998, 253.51000000000005, 332.48924999999997, 400.26625, 259.199, 193.57524999999998,
			//	246.79199999999997, 126.34625000000001, 446.93725000000006, 284.5105000000001, 228.69874999999993,
			//	222.83600000000004, 76.84925, 125.64399999999999, 223.31750000000002, 159.86124999999998,
			//	96.63099999999997, 214.03700000000006, 58.40474999999999, 98.26274999999998, 77.83399999999997,
			//	183.54325, 99.67825000000003, 97.7255, 65.80775, 156.72074999999998, 248.05975, 211.10025,
			//	30.591999999999985, 102.40125, 375.77874999999995, 237.28875000000002},
			//Shared: []float64{469.37504066308964, 269.16344118701187, 247.79912833154737, 285.9089988020236, 195.68133973220046,
			//	181.17705611711872, 231.94233091586509, 118.81025569319527, 568.3480380854837, 177.26498187046676,
			//	195.1507067570836, 184.49518626678585, 55.30403635971437, 120.37245908675781, 248.9226964417048,
			//	114.97495242790902, 77.4731909113465, 140.54575380306014, 52.93722182516522, 87.30700316797103,
			//	57.269518143138214, 156.59580556891132, 101.53409347174322, 71.69669062370322, 48.24383385563337,
			//	136.29875153160742, 218.91929080634907, 154.69922406722495, 22.70948831214511, 63.05940100261651,
			//	326.4278928019924, 193.4856913694349},
			//Distributed:   []float64{3586.3287499999997},
			//Produced:      []float64{5573.8935},
			TotalProduced: 5573.8935}

		assert.Equal(t, expectedReport, report)
		assert.Equal(t, expectedResult[0], results[0])
		assert.Equal(t, expectedResult[1], results[1])

		fmt.Printf("Duration: %f\n", time.Since(begin).Seconds())
	})

	t.Run("calculate row only report", func(t *testing.T) {
		am, cm, pm, dm, sm, ps := calculation.CalculateEEG(db, fmt.Sprintf("%s/%.2d/18/05/30/00", "2021", 4))
		fmt.Printf("Production Sum: %+v\n", ps)
		require.NotNil(t, am)
		require.NotNil(t, cm)
		require.NotNil(t, pm)
		require.NotNil(t, dm)
		require.NotNil(t, sm)

		fmt.Printf("Allocation: %+v\n", am)
		fmt.Printf("Consumtion: %+v\n", cm)
		fmt.Printf("Production: %+v\n", pm)

		expectedConsumption := []float64{0.155750, 0.016250, 0.152750, 0.026500, 0.047500, 0.021750, 0.059000,
			0.019000, 0.048250, 0.099750, 0.016750, 0.029000, 0.003750, 0.008000, 0.012250, 0.022500, 0.023000,
			0.075500, 0.002250, 0.036500, 0.007750, 0.010250, 0.005500, 0.030000, 0.008250, 0.059750, 0.030250,
			0.004250, 0.021000, 0.015750, 0.041250, 0.066250}

		assert.ElementsMatch(t, cm.Elements, expectedConsumption)
		require.True(t, ps == 0.0)

	})

	//t.Run("calculate monthly dashboards", func(t *testing.T) {
	//	err = calculation.CalculateMonthlyDash(db, fmt.Sprintf("%d", 2021), calculation.CalculateEEG)
	//	require.Nil(t, err)
	//})
}

func TestCalculateBadImport(t *testing.T) {
	db, err := store.OpenStorageTest("excelsource", "ecIdTest", "../test/rawdata")
	require.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("../test/rawdata/excelsource")
	}()

	//empty := true
	for _, _ = range ImportTestContent(t, "./221220 Daten VIERE 04-10 bis 18-12.xlsx", "Energiedaten", db) {
		//err = calculation.CalculateMonthlyDash(db, fmt.Sprintf("%d", k), calculation.CalculateEEG)
		//require.NoError(t, err)
		//empty = false
	}
	//require.Equal(t, false, empty)

	t.Run("Calc yearly report", func(t *testing.T) {
		results, report, err := calculation.CalculateYearlyReport(db, 2022, calculation.CalculateEEG)
		require.NoError(t, err)
		require.NotNil(t, results)
		require.NotNil(t, report)
		require.NotNil(t, report.Allocated)

		fmt.Printf("Result: %+v\n", results)
		fmt.Printf("Allocated: %+v\n", report.Allocated)
		fmt.Printf("Consumed:  %+v\n", report.Consumed)
		fmt.Printf("TotalProduced:  %+v\n", report.TotalProduced)
	})
}
