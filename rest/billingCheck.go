package rest

import (
	"encoding/json"
	"net/http"

	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

// fetchBillingCheck returns the data checks before a billing run (platform#148): L3 values,
// first/last data per metering point and community totals for the requested period. Read only.
func fetchBillingCheck() middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		ecid := mux.Vars(r)["ecid"]

		var request model.BillingCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := request.Validate(); err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := store.QueryBillingCheck(tenant, ecid, &request)
		if err != nil {
			glog.Errorf("billing check failed: %v tenant=%s", err, tenant)
			respondWithError(w, http.StatusInternalServerError, "billing check failed")
			return
		}
		respondWithJSON(w, http.StatusOK, resp)
	}
}
