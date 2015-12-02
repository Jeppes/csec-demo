package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"log"
	"net/http"
	"time"
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
	http.HandleFunc("/account", account)
	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type User struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Name    string        `json:"name", bson:"name,omitempty"`
	Balance int           `bson:"balance,omitempty"`
}

type Transaction struct {
	Message string        `bson:"message",omitempty`
	Amount  int           `bson:"amount"`
	From    bson.ObjectId `bson:"from"`
	To      bson.ObjectId `bson:"to"`
	Date    time.Time     `bson:"time"`
}

type Account struct {
	User         User
	Transactions template.HTML
}

func generateHtml(transactions []Transaction) {

}

func account(w http.ResponseWriter, r *http.Request) {
	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	name := userSession.Values["user"].(string)
	if name == "" {
		http.Redirect(w, r, "/", 401)
	}

	session := s.session.Copy()
	defer session.Close()

	var user User
	users := session.DB("csec-demo").C("users")
	err = users.Find(User{Name: name}).One(&user)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var transactions []Transaction
	cTransactions := session.DB("csec-demo").C("transactions")
	err = cTransactions.Find(bson.M{"$or": []bson.M{bson.M{"from": user.ID}, bson.M{"to": user.ID}}}).All(&transactions)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var message string
	var buffer bytes.Buffer
	for _, transaction := range transactions {
		if transaction.From == user.ID {
			message = fmt.Sprintf("to %s", transaction.To)
		} else {
			message = fmt.Sprintf("from %s", transaction.From)
		}
		buffer.WriteString(fmt.Sprintf(
			`<li>
				<span class="date"><i>%s</i></span>
				<br>	
				<strong>%s</strong>%s
			</li>`, transaction.Date, transaction.Amount, message))
	}

	account := Account{User: user, Transactions: template.HTML(buffer.String())}

	tmpl, err := template.New("account.html").ParseFiles("account.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	tmpl.Execute(w, account)

	log.Println("Account page generate for user:", user.Name)
}

func login(w http.ResponseWriter, r *http.Request) {
	var user User

	name := r.FormValue("name")

	log.Println("User:", name)

	session := s.session.Copy()
	defer session.Close()

	users := session.DB("csec-demo").C("users")

	err := users.Find(User{Name: name}).One(&user)
	if err != nil {
		user = User{ID: bson.NewObjectId(), Name: user.Name, Balance: 1000}
		err = users.Insert(user)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Println("User", user.Name, "created with id:", user.ID)
	}

	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	userSession.Values["user"] = user.Name
	userSession.Save(r, w)

	log.Println("User:", user.Name, "logged in!")

	http.Redirect(w, r, "/account", 302)
}
