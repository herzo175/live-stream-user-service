package billing

import (
	"github.com/gorilla/mux"
	"errors"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/util/requests"
	"log"
	"github.com/robfig/cron"

	"github.com/herzo175/live-stream-user-service/src/bundles/users"
)

type BillingController struct {
	logic BillingLogic
}

func MakeRouter(masterRouter *mux.Router, logic BillingLogic) {
	controller := BillingController{}
	subrouter := masterRouter.PathPrefix("/billing").Subrouter()

	subrouter.HandleFunc(
		"/events", requests.GetAuthenticated(&users.UserTokenBody{}, controller.Get),
	).Methods("GET")

	controller.logic = logic

	// start cron to aggregate payments on running servers (run every day at midnight)
	billingCron := cron.New()
	// NOTE: do not run more than once in an hour
	billingCron.AddFunc("0 0 0 * * *", func() {
		log.Println("Starting ChargeAll cron job")
		// TODO: fault tolerance for cron job (and use our boy's service at cronhub)
		controller.logic.ChargeAll()
		log.Println("Finished ChargeAll cron job")
	})

	log.Println("starting billing cron jobs")
	billingCron.Start()
}

func (controller *BillingController) Get(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, *requests.ControllerError) {
	start, end, err := database.ExtractStartEnd(queryParams)

	if err != nil {
		log.Printf("Failed to extract start and end: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("Failed to extract start and end of results"),
		}
	}

	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	filter, err := database.TranslateQueryMap(queryParams, BillingEvent{})

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("Failed to translate query params to filter expressions"),
		}
	}

	paginatedList, controllerError := controller.logic.Get(userId, filter, int(start), int(end))
	return &paginatedList, controllerError
}
