package billing

import (
	"github.com/lib/pq"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/util/requests"

	"github.com/herzo175/live-stream-user-service/src/util/payments"
)

// TODO: point to user group in the future
type BillingEvent struct {
	Id             string    `json:"_id" gorm:"column:_id"`
	RelatedUnitId  string    `json:"related_unit_id" gorm:"column:related_unit_id"`
	UserId         string    `json:"user_id" gorm:"column:user_id"`
	PlanId         string    `json:"plan_id" gorm:"column:plan_id"`
	InvoiceItemId  string    `json:"invoice_item_id" gorm:"column:invoice_item_id"`
	PaymentEventId string    `json:"payment_event_id" gorm:"column:payment_event_id"`
	AmountBilled   float64   `json:"amount_billed" gorm:"column:amount_billed"`
	Start          time.Time `json:"start" gorm:"column:start_time"`
	End            pq.NullTime `json:"end" gorm:"column:stop_time"`
}

func (BillingEvent) TableName() string {
	return "public.live_stream_billing_events"
}

type BillingLogic interface {
	Start(userId, planId, relatedUnitId string) *requests.ControllerError
	Stop(relatedUnitId string) *requests.ControllerError
	Get(userId string, filter map[string]interface{}, start, end int) (*database.PaginatedList, *requests.ControllerError)
	Charge(events ...*BillingEvent) *requests.ControllerError
	ChargeAll()
}

type BillingLogicConfig struct {
	db *database.SQLDatabaseClient
	// collection  *mgo.Collection
	userLogic *users.UserLogic
}

func MakeBillingLogicConfig(db *database.SQLDatabaseClient, userLogic *users.UserLogic) *BillingLogicConfig {
	config := BillingLogicConfig{}
	config.db = db
	config.userLogic = userLogic
	return &config
}

func (config *BillingLogicConfig) Start(userId, planId, relatedUnitId string) *requests.ControllerError {
	event := BillingEvent{}

	event.Id = uuid.New().String()
	event.RelatedUnitId = relatedUnitId
	event.UserId = userId
	event.PlanId = planId
	event.Start = time.Now()

	err := config.db.Create(&event)

	if err != nil {
		log.Printf("Failed to create new billing event for user %s: %v", userId, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      errors.New("Failed to create new billing event"),
		}
	}

	return nil
}

func (config *BillingLogicConfig) Get(
	userId string,
	filter map[string]interface{},
	start, end int,
) (*database.PaginatedList, *requests.ControllerError) {
	filter["user_id"] = userId

	billingEvents := new([]BillingEvent)
	total, err := config.db.FindMany(billingEvents, start, end, filter)

	if err != nil {
		log.Printf("Error retrieving billing events for user %s: %v", userId, err)
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error:      errors.New("Could not find billing events"),
		}
	}

	return database.ToPaginatedList(*billingEvents, start, end, total), nil
}

func (config *BillingLogicConfig) Stop(relatedUnitId string) *requests.ControllerError {
	events := new([]*BillingEvent)
	_, err := config.db.FindMany(
		events, -1, -1, "related_unit_id = ? AND stop_time IS NULL", relatedUnitId,
	)

	if err != nil {
		log.Printf("Could not find billing events by related unit %s: %v", relatedUnitId, err)
		return &requests.ControllerError{
			StatusCode: 404,
			Error:      fmt.Errorf("Could not find billing events for unit %s", relatedUnitId),
		}
	}

	for _, event := range *events {
		eventUpdate := make(map[string]interface{})
		eventUpdate["stop_time"] = time.Now()
		err = config.db.Update(event, eventUpdate, "_id = ?", event.Id)

		if err != nil {
			log.Printf("Could not stop billing for %s, event %s: %v", relatedUnitId, event.Id, err)
			return &requests.ControllerError{
				StatusCode: 500,
				Error:      fmt.Errorf(
					"Could not stop billing for %s, event %s", relatedUnitId, event.Id,
				),
			}
		}
	}

	return config.Charge(*events...)
}

func (config *BillingLogicConfig) ChargeAll() {
	// find all events without a charge event and group by user id
	results := [][]*BillingEvent{}

	config.db.DB.
		Table("public.live_stream_billing_events").
		Select("*").
		Group("user_id").
		Scan(&results).
		Having("invoice_item_id IS NULL")

	log.Println(fmt.Sprintf("Charging for %v users", len(results)))

	// send group to be charged
	for _, group := range results {
		// TODO: handle error if one group errors
		go config.Charge(group...)
	}
}

// TODO: webhook(s) to acknowledge payments
func (config *BillingLogicConfig) Charge(events ...*BillingEvent) *requests.ControllerError {
	if len(events) < 1 {
		return nil
	}

	// firstUser := users.User{}
	// err := config.db.FindOne(&firstUser, "_ID = ?", events[0].UserId)
	firstUser, controllerError := config.userLogic.GetById(events[0].UserId)

	if controllerError != nil {
		return controllerError
	}

	for _, event := range events {
		// if stop is null, stop event and create new event to compensate for carry over
		eventUpdates := make(map[string]interface{})

		if (!event.End.Valid) {
			fmt.Println("stopping event")
			event.End.Time = time.Now()
			eventUpdates["stop_time"] = event.End.Time

			controllerError := config.Start(event.UserId, event.PlanId, event.RelatedUnitId)

			if controllerError != nil {
				return controllerError
			}
		}

		if controllerError != nil {
			return controllerError
		}

		// calculate units between stops in accordance with rate enum and time
		timeDiff := event.End.Time.Sub(event.Start)
		units := int(math.Ceil(timeDiff.Hours()))

		// create invoice
		stripeCustomer, err := payments.GetCustomer(firstUser.StripeCustomerId)

		if err != nil {
			log.Printf("Unable to retrieve customer %s: %v", firstUser.StripeCustomerId, err)
			return &requests.ControllerError{
				StatusCode: 500,
				Error:      errors.New("Unable to retrieve billing customer information"),
			}
		}

		var subscriptionItemId string

		for _, sub := range stripeCustomer.Subscriptions.Data {
			if sub.Plan.ID == event.PlanId {
				// NOTE: assumes that subscribing a customer to the plan will yield one subscription item
				subscriptionItemId = sub.Items.Data[0].ID
			}
		}

		if subscriptionItemId == "" {
			return &requests.ControllerError{
				StatusCode: 400,
				Error:      errors.New("Item subscription plan could not be found"),
			}
		}

		invoiceItem, err := payments.AddToInvoice(subscriptionItemId, int64(units))

		// add charge event to event
		if err != nil {
			log.Printf("Failed to add subscription item to invoice: %v", err)
			return &requests.ControllerError{
				StatusCode: 500,
				Error:      errors.New("Failed to add subscription item to invoice"),
			}
		}

		eventUpdates["invoice_item_id"] = invoiceItem.ID

		plan, _ := payments.GetPlan(event.PlanId)

		eventUpdates["amount_billed"] = float64(invoiceItem.Quantity) * (float64(plan.Amount) / 100)

		// save event
		err = config.db.Update(event, eventUpdates, "_id = ?", event.Id)

		if err != nil {
			log.Printf("An error occured while updating billing event event %s: %v", event.Id, err)
			return &requests.ControllerError{
				StatusCode: 500,
				Error:      errors.New("Failed to update billing event"),
			}
		}
	}

	// TODO: credit user for server hours as a monthly coupon

	return nil
}
