package main

import (
	"encoding/json"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type Server struct {
	session *mgo.Session
}

var (
	s     Server
	store = sessions.NewCookieStore([]byte("secret")) // Pls use a real secret
)

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	s.session = session
	defer s.session.Close()

	http.HandleFunc("/login", login)
	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type User struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Name    string        `json:"name", bson:"name,omitempty"`
	Balance int           `bson:"balance,omitempty"`
}

func login(w http.ResponseWriter, r *http.Request) {
	var user User

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := r.Body.Close(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := json.Unmarshal(body, &user); err != nil {
		http.Error(w, err.Error(), 422) // unprocessable entity
	}

	session := s.session.Copy()
	defer session.Close()

	users := session.DB("csec-demo").C("users")

	err = users.Find(User{Name: user.Name}).One(&user)
	if err != nil {
		user = User{ID: bson.NewObjectId(), Name: user.Name, Balance: 1000}
		err = users.Insert(user)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Println("User created with id:", user.ID)
	}

	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	userSession.Values["user"] = user.Name
	userSession.Save(r, w)

	log.Println("User:", user.Name, "logged in!")
}
