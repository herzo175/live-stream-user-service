package billing

import (
	"gopkg.in/mgo.v2"
	"time"

	"gopkg.in/mgo.v2/bson"
)

// TODO: point to user group in the future
type BillingEvent struct {
	Id       bson.ObjectId `json:"_id" bson:"_id,omitemtpy"`
	ServerId bson.ObjectId `json:"server_id" bson:"server_id,omitempty"`
	UserId   bson.ObjectId `json:"user_id" bson:"user_id,omitempty"`
	Start    time.Time     `json:"start" bson:"start"`
	End      time.Time     `json:"end" bson:"end"`
}

type BillingDB struct {
	collection *mgo.Collection
}

func MakeSchema(dbName string, session *mgo.Session) *BillingDB {
	schema := BillingDB{}
	schema.collection = session.DB(dbName).C("BillingEvents")

	return &schema
}
