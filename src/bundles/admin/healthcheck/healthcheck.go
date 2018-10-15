package healthcheck

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/herzo175/live-stream-user-service/src/bundles"
)

type HealthcheckResponse struct {
	Status string `json:"status"`
}

type HealthcheckController struct {
	Controller *bundles.Controller
}

func (controller *HealthcheckController) MakeRouter() {
	subrouter := controller.Controller.Router.PathPrefix("/healthcheck").Subrouter()
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
