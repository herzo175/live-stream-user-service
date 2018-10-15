package bundles

import (
	"github.com/gorilla/mux"
	mgo "gopkg.in/mgo.v2"
)

type Controller struct {
	Router   *mux.Router
	DBClient *mgo.Session
	DBName   string
}
