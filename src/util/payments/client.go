package payments

import (
	"github.com/stripe/stripe-go/source"
	"github.com/stripe/stripe-go/plan"
	"github.com/stripe/stripe-go/usagerecord"
	"time"
	"github.com/stripe/stripe-go/sub"
	"fmt"
	"github.com/stripe/stripe-go/paymentsource"
	"github.com/stripe/stripe-go/token"
	"os"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
)

// NOTE: make interface if mocking is needed for unit testing

func CreateCustomer(email string) (cust *stripe.Customer, err error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	fmt.Println(stripe.Key)

	newCustomerParams := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	cust, err = customer.New(newCustomerParams)

	if err != nil {
		return nil, err
	}

	// subscribe customer to all plans
	addSubscriptionParams := &stripe.SubscriptionParams{
		Customer: stripe.String(cust.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Plan: stripe.String(os.Getenv("STRIPE_SERVER_PLAN_ID")),
			},
		},
	}

	_, err = sub.New(addSubscriptionParams)

	return cust, err
}

func AddSource(cardNumber, expMonth, expYear, cvc, stripeCustomerId string) (*stripe.PaymentSource, error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	tokenParams := &stripe.TokenParams{
		Card: &stripe.CardParams{
			Number: stripe.String(cardNumber),
			ExpMonth: stripe.String(expMonth),
			ExpYear: stripe.String(expYear),
			CVC: stripe.String(cvc),
		},
	}

	t, err := token.New(tokenParams)

	if err != nil {
		return nil, err
	}

	customerSourceParams := &stripe.CustomerSourceParams{
		Customer: stripe.String(stripeCustomerId),
		Source: &stripe.SourceParams{
			Token: stripe.String(t.ID),
		},
	}

	return paymentsource.New(customerSourceParams)
}

func GetCustomer(stripeCustomerId string) (*stripe.Customer, error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	return customer.Get(stripeCustomerId, nil)
}

func DeleteCustomer(stripeCustomerId string) error {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	params := &stripe.CustomerParams{}
	_, err := customer.Del(stripeCustomerId, params)
	return err
}

func GetPlan(planId string) (*stripe.Plan, error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	return plan.Get(planId, nil)
}

func AddToInvoice(subscriptionItemId string, units int64) (*stripe.UsageRecord, error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	params := &stripe.UsageRecordParams{
		Quantity: stripe.Int64(units),
		Timestamp: stripe.Int64(time.Now().Unix()),
		SubscriptionItem: stripe.String(subscriptionItemId),
	}
	
	return usagerecord.New(params)
}

func DetatchSource(customerId, sourceId string) (err error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	params := &stripe.SourceObjectDetachParams{
		Customer: stripe.String(customerId),
	}

	_, err = source.Detach(sourceId, params)
	return err
}

func SetDefaultSource(customerId, sourceId string) (err error) {
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	params := &stripe.CustomerParams{
		DefaultSource: stripe.String(sourceId),
	}

	_, err = customer.Update(customerId, params)
	return err
}
