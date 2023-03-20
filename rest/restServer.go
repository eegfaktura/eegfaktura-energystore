package rest

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
)

func NewRestServer() *mux.Router {
	jwtWrapper := middleware.JWTMiddleware(viper.GetString("jwt.pubKeyFile"))
	r := mux.NewRouter()
	//s := r.PathPrefix("/rest").Subrouter()
	r.HandleFunc("/eeg/{year}/{month}", jwtWrapper(fetchEnergy())).Methods("GET")
	r.HandleFunc("/eeg/hello", getHello).Methods("GET")
	return r
}

func getHello(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"sepp": "hello"})
}

func fetchEnergy() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		var err error
		var year, month int
		vars := mux.Vars(r)
		energy := &model.EegEnergy{}

		fc := ""
		year, err = strconv.Atoi(vars["year"])
		month, err = strconv.Atoi(vars["month"])

		fmt.Printf("FETCH DASHBOARD: %+v %+v (%v/%v) \n", tenant, claims, year, month)

		if energy, err = calculation.EnergyDashboard(tenant, fc, year, month); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := struct {
			Eeg *model.EegEnergy `json:"eeg"`
		}{Eeg: energy}

		respondWithJSON(w, http.StatusOK, &resp)
	}
}
