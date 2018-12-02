package healthcheck

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type HealthcheckResponse struct {
	Status string `json:"status"`
}

func MakeRouter(masterRouter *mux.Router) {
	subrouter := masterRouter.PathPrefix("/healthcheck").Subrouter()
	subrouter.HandleFunc("", getHealthStatus)
}

func getHealthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	err := encoder.Encode(checkStatus())

	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
}

func checkStatus() *HealthcheckResponse {
	status := HealthcheckResponse{}

	status.Status = "happy"

	return &status
}
