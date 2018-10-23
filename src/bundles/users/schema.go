package users

import (
	"log"
	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"golang.org/x/crypto/bcrypt"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// TODO: update billing info
// TODO: reset password, email
// TODO: permission groups
// TODO: user create, user update structs
type User struct {
	Id       bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	Email    string        `json:"email" bson:"email,omitempty"`
	Password string        `json:"password" bson:"password,omitempty"`
	Roles []auth.Permission `json:"roles" bson:"roles,omitempty"`
}

type UserTokenBody struct {
	Id    string            `json:"_id"`
	Roles []auth.Permission `json:"roles"`
}

func (t UserTokenBody) HasPermission(service, role string) bool {
	for _, r := range t.Roles {
		if r.Service == service && r.Role == role {
			return true
		}
	}

	return false
}

type Schema struct {
	collection *mgo.Collection
}

func MakeSchema(dbName string, session *mgo.Session) *Schema {
	schema := Schema{}
	schema.collection = session.DB(dbName).C("Users")

	// TODO: notify if duplicate key
	index := mgo.Index{
		Key:      []string{"email"},
		Unique:   true,
		DropDups: true,
	}

	err := schema.collection.EnsureIndex(index)

	if err != nil {
		log.Fatal(err)
	}

	return &schema
}

func (schema *Schema) Register(user *User) (token auth.TokenResponse, err error) {
	// TODO: validate user (ex. check if password is good)
	// TODO: set up billing info for user
	// TODO: confirm email mechanism
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)

	if err != nil {
		return token, err
	}

	user.Password = string(hash)

	user.Id = bson.NewObjectId()
	err = schema.collection.Insert(user)

	if err != nil {
		return token, err
	}

	return auth.GenerateToken(user)
}

func (schema *Schema) Login(email, plaintext string) (token auth.TokenResponse, err error) {
	user := User{}
	err = schema.collection.Find(bson.M{"email": email}).One(&user)

	if err != nil {
		return token, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(plaintext))

	if err != nil {
		return token, err
	}

	return auth.GenerateToken(user)
}

func (schema *Schema) GetById(id string) (user *User, err error) {
	user = &User{}
	err = schema.collection.Find(bson.M{"_id": bson.ObjectIdHex(id)}).One(user)

	if err != nil {
		return user, err
	}

	return user, nil
}
