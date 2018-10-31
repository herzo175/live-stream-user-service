package billing

import (
	"log"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/herzo175/live-stream-user-service/src/bundles/users"

	"github.com/herzo175/live-stream-user-service/src/util/payments"

	"github.com/herzo175/live-stream-user-service/src/util/querying"

	"gopkg.in/mgo.v2"

	"gopkg.in/mgo.v2/bson"
)

// TODO: point to user group in the future
type BillingEvent struct {
	Id             bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	RelatedUnitId  bson.ObjectId `json:"related_unit_id" bson:"related_unit_id"`
	UserId         bson.ObjectId `json:"user_id" bson:"user_id"`
	PlanId         string        `json:"plan_id" bson:"plan_id"`
	InvoiceItemId  string        `json:"invoice_item_id" bson:"invoice_item"`
	PaymentEventId string        `json:"payment_event_id" bson:"payment_event"`
	AmountBilled   float64       `json:"amount_billed" bson:"amount_billed"`
	Start          time.Time     `json:"start" bson:"start"`
	End            time.Time     `json:"end" bson:"end"`
}

type BillingDB struct {
	collection  *mgo.Collection
	usersSchema *users.Schema
}

func MakeSchema(db *mgo.Database) *BillingDB {
	schema := BillingDB{}
	// TODO: put schema names in config
	// TODO: make schemas more accessable
	schema.collection = db.C("BillingEvents")
	schema.usersSchema = users.MakeSchema(db)

	return &schema
}

func (schemaConfig *BillingDB) Start(userId bson.ObjectId, planId string, relatedUnitId bson.ObjectId) error {
	event := BillingEvent{}

	event.Id = bson.NewObjectId()
	event.RelatedUnitId = relatedUnitId
	event.UserId = userId
	event.PlanId = planId
	event.Start = time.Now()

	return schemaConfig.collection.Insert(&event)
}

func (schemaConfig *BillingDB) Get(userId string, filter map[string]interface{}, start, end int) (results *querying.PaginatedList, err error) {
	clauses := make(map[string]interface{})

	for k, v := range filter {
		clauses[k] = v
	}

	clauses["user_id"] = bson.ObjectIdHex(userId)

	return querying.GetPaginatedList(schemaConfig.collection, &[]BillingEvent{}, start, end, clauses)
}

func (schemaConfig *BillingDB) Stop(relatedUnitId string) (err error) {
	event := BillingEvent{}
	query := bson.M{"related_unit_id": bson.ObjectIdHex(relatedUnitId)}

	err = schemaConfig.collection.Find(query).One(&event)

	if err != nil {
		return err
	}

	return schemaConfig.Charge(event.UserId, event)
}

func (schemaConfig *BillingDB) ChargeAll() {
	// find all events without a charge event and group by user id
	type aggregateResult struct {
		UserId bson.ObjectId  `bson:"_id"`
		Events []BillingEvent `bson:"events"`
	}

	results := []aggregateResult{}

	pipeline := []bson.M{
		bson.M{
			"$match": bson.M{"invoice_item_id": nil},
		},
		bson.M{
			"$group": bson.M{
				"_id": "$user_id",
				"events": bson.M{
					"$push": "$$ROOT",
				},
			},
		},
	}

	pipe := schemaConfig.collection.Pipe(pipeline)
	pipe.All(&results)

	log.Println(fmt.Sprintf("Charging for %v users", len(results)))

	// send group to be charged
	for _, group := range results {
		go schemaConfig.Charge(group.UserId, group.Events...)
	}
}

// TODO: webhook(s) to acknowledge payments
func (schemaConfig *BillingDB) Charge(userId bson.ObjectId, events ...BillingEvent) (err error) {
	user, err := schemaConfig.usersSchema.GetById(userId.Hex())

	if err != nil {
		// TODO: thread based error handling
		return err
	}

	for _, event := range events {
		// if stop is null, stop event and create new event to compensate for carry over
		if (event.End == time.Time{}) {
			fmt.Println("stopping event")
			event.End = time.Now()

			err = schemaConfig.Start(event.UserId, event.PlanId, event.RelatedUnitId)

			if err != nil {
				return err
			}
		}

		// calculate units between stops in accordance with rate enum and time
		timeDiff := event.End.Sub(event.Start)
		units := int(math.Ceil(timeDiff.Hours()))

		// create invoice
		stripeCustomer, err := payments.GetCustomer(user.StripeCustomerId)

		if err != nil {
			return err
		}

		var subscriptionItemId string

		for _, sub := range stripeCustomer.Subscriptions.Data {
			if sub.Plan.ID == event.PlanId {
				// NOTE: assumes that subscribing a customer to the plan will yield one subscription item
				subscriptionItemId = sub.Items.Data[0].ID
			}
		}

		if subscriptionItemId == "" {
			return errors.New(fmt.Sprintf("Customer subscription to plan %s could not be found", event.PlanId))
		}

		invoiceItem, err := payments.AddToInvoice(subscriptionItemId, int64(units))

		// add charge event to event
		if err != nil {
			return err
		}

		event.InvoiceItemId = invoiceItem.ID

		plan, _ := payments.GetPlan(event.PlanId)

		event.AmountBilled = float64(invoiceItem.Quantity) * (float64(plan.Amount) / 100)

		// save event
		query := bson.M{"_id": event.Id}
		err = schemaConfig.collection.Update(query, event)

		if err != nil {
			return err
		}
	}

	// TODO: credit user for server hours as a monthly coupon

	return nil
}
