package billing

import (
	"log"
	"github.com/robfig/cron"

	"github.com/herzo175/live-stream-user-service/src/util/querying"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
	"github.com/herzo175/live-stream-user-service/src/bundles"
)

type BillingController struct {
	Controller *bundles.Controller
	schema     *BillingDB
}

func (controller *BillingController) MakeRouter() {
	subrouter := controller.Controller.Router.PathPrefix("/billing").Subrouter()

	subrouter.HandleFunc(
		"/events", bundles.GetAuthenticated(&users.UserTokenBody{}, controller.Get),
	).Methods("GET")

	controller.schema = MakeSchema(controller.Controller.DB)

	// start cron to aggregate payments on running servers (run every day at midnight)
	billingCron := cron.New()
	// NOTE: do not run more than once in an hour
	billingCron.AddFunc("0 0 0 * * *", func() {
		log.Println("Starting ChargeAll cron job")
		// TODO: fault tolerance for cron job (and use our boy's service at cronhub)
		controller.schema.ChargeAll()
		log.Println("Finished ChargeAll cron job")
	})

	log.Println("starting billing cron jobs")
	billingCron.Start()
}

func (controller *BillingController) Get(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, error) {
	start, end, err := querying.ExtractStartEnd(queryParams)

	if err != nil {
		return nil, err
	}

	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	filter, err := querying.GenerateQueryFromMultivaluedMap(queryParams, BillingEvent{})

	if err != nil {
		return nil, err
	}

	paginatedList, err := controller.schema.Get(userId, filter, int(start), int(end))
	return &paginatedList, err
}
